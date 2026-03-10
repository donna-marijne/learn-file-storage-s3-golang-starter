package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

type FFProbeResult struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	var cmdOutput bytes.Buffer
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)
	cmd.Stdout = &cmdOutput
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("ffprobe failed: %w", err)
	}

	ffprobeResult := FFProbeResult{}
	err = json.Unmarshal(cmdOutput.Bytes(), &ffprobeResult)
	if err != nil {
		return "", fmt.Errorf("unmarshal ffprobe stdout failed: %w", err)
	}
	if len(ffprobeResult.Streams) == 0 {
		return "", fmt.Errorf("ffprobe identified no streams")
	}

	stream := ffprobeResult.Streams[0]

	aspectRatio := getAspectRatio(float64(stream.Width), float64(stream.Height))

	return aspectRatio, nil
}

func getAspectRatio(w, h float64) string {
	const landscape = 16.0 / 9.0
	const portrait = 9.0 / 16.0

	ratio := w / h

	if math.Abs(ratio-landscape) < 0.1 {
		return "16:9"
	}

	if math.Abs(ratio-portrait) < 0.1 {
		return "9:16"
	}

	return "other"
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"
	cmd := exec.Command(
		"ffmpeg",
		"-i",
		filePath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		outputFilePath,
	)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("ffmpeg processing failed: %w", err)
	}

	return outputFilePath, nil
}
