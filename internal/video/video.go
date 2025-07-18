package video

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
)

type videoStream struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}
type videoIndex struct {
	Streams []videoStream `json:"streams"`
}

func GetVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", errors.New("failed to run command")
	}

	dataBytes := out.Bytes()
	var vIndex videoIndex
	err = json.Unmarshal(dataBytes, &vIndex)
	if err != nil {
		return "", errors.New("failed unmarshal JSON")
	}

	var vWidth int
	var vHeight int
	var vCalc float32
	var vAspectRatio string

	if len(vIndex.Streams) > 0 {
		vWidth = vIndex.Streams[0].Width
		vHeight = vIndex.Streams[0].Height
	} else {
		return "", errors.New("unable to find full width and height details about the video")
	}

	vCalc = float32(vWidth) / float32(vHeight)
	if vCalc >= 1.7 && vCalc <= 1.8 {
		vAspectRatio = "landscape"
	} else if vCalc >= 0.5 && vCalc <= 0.6 {
		vAspectRatio = "portrait"
	} else {
		vAspectRatio = "other"
	}

	return vAspectRatio, nil
}

func ProcessVideoFastStart(filepath string) (string, error) {
	newFilePath := filepath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", newFilePath)
	err := cmd.Run()
	if err != nil {
		return "", errors.New("failed to move faststart flags in the file")
	}

	return newFilePath, nil
}

/*
func GeneratePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)

	req, err := presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expireTime))

	if err != nil {
		return "", errors.New("failed to presign request" + err.Error())
	}

	return req.URL, nil
}
*/
