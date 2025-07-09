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

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	dataBytes := out.Bytes()
	var vIndex videoIndex
	err = json.Unmarshal(dataBytes, &vIndex)
	if err != nil {
		return "", err
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
		vAspectRatio = "16:9"
	} else if vCalc >= 0.5 && vCalc <= 0.6 {
		vAspectRatio = "9:16"
	} else {
		vAspectRatio = "other"
	}

	return vAspectRatio, nil
}
