package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/vmilasinovic/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/vmilasinovic/learn-file-storage-s3-golang-starter/internal/video"
)

const MAX_UPLOAD_SIZE = 1 << 30

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// Limit incoming file size
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)

	// Get video ID
	videoIDString := r.PathValue("videoID")
	videoIDUUID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No video with this UUID", err)
		return
	}

	// Authorize user and get it's ID
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	// Get video metadata
	videoMetadata, err := cfg.db.GetVideo(videoIDUUID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to fetch video metadata", err)
		return
	}

	// Check if the user is the file's owner
	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "This user is not the file owner", err)
		return
	}

	// Check the file and it's size
	err = r.ParseMultipartForm(MAX_UPLOAD_SIZE)
	if err != nil {
		if err == http.ErrMissingFile {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		if err.Error() == "http: request body too large" {
			http.Error(w, fmt.Sprintf("Video too large. Max size is %d bytes.", MAX_UPLOAD_SIZE), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Parse the uploaded video from data
	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		http.Error(w, "Error retrieving file from form: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check for file's type
	fileMediaType := fileHeader.Header.Get("Content-Type")
	mediatype, _, err := mime.ParseMediaType(fileMediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get file type", nil)
		return
	}

	if mediatype != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", err)
		return
	}

	// Save the uploaded file to tmp file on disk
	f, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create a tmp file for upload", err)
		return
	}

	_, err = io.Copy(f, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying file content", err)
		return
	}

	// Reset the tmp file's file pointer to the begining - this will alow reading the file again from the begining
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to move the tmp file's pointer to the beginning", err)
		return
	}

	// Put the file in the bucket
	bts := make([]byte, 32)
	_, err = rand.Read(bts)
	videoNewFilename := base64.RawURLEncoding.EncodeToString(bts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create videoIDString", err)
		return
	}

	fileKeyPrefix, err := video.GetVideoAspectRatio(f.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video aspect ratio prefix: "+err.Error(), err)
		return
	}

	processedTmpFile, err := video.ProcessVideoFastStart(f.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to preprocess TMP file: "+err.Error(), err)
		return
	}

	err = f.Close()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to close temp file: "+err.Error(), err)
		return
	}
	err = os.Remove(f.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to clean temp file: "+err.Error(), err)
		return
	}

	newFilename := strings.Replace(processedTmpFile, ".processing", "", 1)
	err = os.Rename(processedTmpFile, newFilename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to rename processed TMP file: "+err.Error(), err)
		return
	}

	newF, err := os.Open(newFilename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to open processed TMP file: "+err.Error(), err)
		return
	}
	defer os.Remove(newFilename)
	defer newF.Close()

	fileKey := fileKeyPrefix + "/" + videoNewFilename + ".mp4"

	s3Params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        newF,
		ContentType: &mediatype,
	}

	_, err = cfg.s3Client.PutObject(context.Background(), &s3Params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to copy file's tmp data into bucket", err)
		return
	}

	// Update the file's URL in the db
	videoURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + fileKey

	updatedVideo := videoMetadata
	updatedVideo.VideoURL = &videoURL

	if err = cfg.db.UpdateVideo(updatedVideo); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed update video data in db", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updatedVideo)
}
