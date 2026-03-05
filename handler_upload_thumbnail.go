package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
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

	// 20MB
	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid upload", err)
		return
	}

	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing 'thumbnail' key", err)
		return
	}

	mediaType := fileHeader.Header.Get("Content-Type")
	data, err := io.ReadAll(file)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	fileName, err := getThumbnailFileName(videoID, mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Cannot save thumbnail", err)
		return
	}

	_, err = cfg.writeThumbnailToFile(fileName, data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Cannot save thumbnail", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	video.ThumbnailURL = &thumbnailURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func getThumbnailFileName(videoID uuid.UUID, mediaType string) (string, error) {
	exts, err := mime.ExtensionsByType(mediaType)
	if err != nil {
		return "", err
	}
	if len(exts) == 0 {
		return "", fmt.Errorf("unsupported content type: %s", mediaType)
	}

	fileName := fmt.Sprintf("%s%s", videoID, exts[0])

	return fileName, nil
}

func (cfg *apiConfig) writeThumbnailToFile(fileName string, data []byte) (string, error) {
	filePath := filepath.Join(cfg.assetsRoot, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create a file at '%s': %v", filePath, err)
	}

	reader := bytes.NewReader(data)
	written, err := io.Copy(file, reader)
	if err != nil {
		return "", fmt.Errorf("failed to write to a file at '%s': %v", filePath, err)
	}

	log.Printf("Wrote %d bytes to '%s'", written, filePath)

	return filePath, nil
}
