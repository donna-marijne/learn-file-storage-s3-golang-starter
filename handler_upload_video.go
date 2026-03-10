package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxBytes = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing 'video' key", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Media type not allowed", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create a temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	bytesWritten, err := io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to write to a temporary file", err)
		return
	}

	log.Printf("Wrote %d bytes to %s", bytesWritten, tempFile.Name())

	// _, err = tempFile.Seek(0, io.SeekStart)
	// if err != nil {
	// 	respondWithError(w, http.StatusInternalServerError, "Failed to seek", err)
	// 	return
	// }

	fastStartFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process video for fast start", err)
		return
	}
	defer os.Remove(fastStartFilePath)

	log.Printf("Processed %s to %s", tempFile.Name(), fastStartFilePath)

	fastStartFile, err := os.Open(fastStartFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to open the fast start video file", err)
		return
	}
	defer fastStartFile.Close()

	aspectRatio, err := getVideoAspectRatio(fastStartFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to probe the video", err)
		return
	}

	fileKey, err := getVideoFileKey(aspectRatio)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate an object key", err)
		return
	}

	putObjectParams := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        fastStartFile,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &putObjectParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "PutObject failed", err)
		return
	}

	videoBucketAndKey := fmt.Sprintf("%s,%s", cfg.s3Bucket, fileKey)
	video.VideoURL = &videoBucketAndKey
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "UpdateVideo failed", err)
		return
	}

	log.Printf("Uploaded %s to %s", fastStartFilePath, videoBucketAndKey)

	video, err = cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get a signed URL", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func getVideoFileKey(aspectRatio string) (string, error) {
	var prefix string
	switch aspectRatio {
	case "16:9":
		prefix = "landscape"
	case "9:16":
		prefix = "portrait"
	default:
		prefix = "other"
	}

	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("RNG error: %v", err)
	}

	fileNameWithoutExt := base64.RawURLEncoding.EncodeToString(randomBytes)

	fileName := fmt.Sprintf("%s/%s.mp4", prefix, fileNameWithoutExt)

	return fileName, nil
}
