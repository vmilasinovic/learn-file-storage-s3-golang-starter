package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/vmilasinovic/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/vmilasinovic/learn-file-storage-s3-golang-starter/internal/database"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")

	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	maxMemory := 10 << 20
	if err = r.ParseMultipartForm(int64(maxMemory)); err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse multipart/form data", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not get thumbnail data from file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnail format", nil)
		return
	}

	bts := make([]byte, 32)
	_, err = rand.Read(bts)
	thumbnailNewFilename := base64.RawURLEncoding.EncodeToString(bts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create videoIDString", err)
		return
	}

	mt := r.PathValue("Content-Type")
	assetPath := filepath.Join(cfg.assetsRoot, thumbnailNewFilename, mt)
	newFile, err := os.Create(assetPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create a new file", err)
		return
	}
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy to a new file", err)
		return
	}

	thumbnailURL := "http://localhost:" + cfg.port + "/" + assetPath

	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}
	if videoID != videoMetadata.ID {
		respondWithError(w, http.StatusForbidden, "Unauthorized user accessing video", err)
		return
	}

	vidUpdateParams := database.Video{
		ID:           videoID,
		ThumbnailURL: &thumbnailURL,
		VideoURL:     videoMetadata.VideoURL,
		CreateVideoParams: database.CreateVideoParams{
			UserID:      videoMetadata.UserID,
			Title:       videoMetadata.Title,
			Description: videoMetadata.Description,
		},
	}
	if err = cfg.db.UpdateVideo(vidUpdateParams); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video parameters", err)
		return
	}

	updatedVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updatedVideo)
}
