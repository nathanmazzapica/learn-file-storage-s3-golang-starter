package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video for video", videoID, "by user", userID)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized to upload this video", err)
		return
	}

	file, header, err := r.FormFile("video")
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid content type: must be mp4", nil)
		return
	}

	tmp, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create video file", err)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	// Copy the data from the multipart file INTO tmp
	if _, err := io.Copy(tmp, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy video file", err)
		return
	}

	tmp.Seek(0, io.SeekStart)

	processedPath, err := processVideoForFastStart(tmp.Name())
	processedFile, err := os.Open(processedPath)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process video file", err)
		return
	}
	defer os.Remove(processedFile.Name())
	defer processedFile.Close()

	ratio, err := getVideoAspectRatio(processedFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get aspect ratio", err)
		return
	}

	var prefix string

	switch ratio {
	case "16:9":
		prefix = "landscape/"
	case "9:16":
		prefix = "portrait/"
	default:
		prefix = "other/"
	}

	s3Key := prefix + getAssetPath(mediaType)
	fmt.Println(s3Key)
	output, err := cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &s3Key,
		Body:        processedFile,
		ContentType: &mediaType,
	})

	fmt.Println(output)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video to s3", err)
		return
	}
	fmt.Println(output)

	s3Url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, s3Key)

	video.VideoURL = &s3Url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, video)

	fmt.Println("video uploaded")
}

func processVideoForFastStart(filepath string) (string, error) {
	outputFilePath := filepath + ".processing"
	cmd := exec.Command(
		"ffmpeg", "-i", filepath,
		"-c", "copy", "-movflags",
		"faststart", "-f", "mp4",
		outputFilePath,
	)

	if err := cmd.Run(); err != nil {
		return outputFilePath, err
	}

	return outputFilePath, nil
}

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var probe struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		}
	}

	err = json.Unmarshal(buf.Bytes(), &probe)
	if err != nil {
		return "", err
	}

	data := probe.Streams[0]

	const ratio16by9 = 16.0 / 9.0
	const ratio9by16 = 9.0 / 16.0
	const epsilon = 0.01

	ratio := float64(data.Width) / float64(data.Height)

	switch {
	case math.Abs(ratio16by9-ratio) <= epsilon:
		return "16:9", nil
	case math.Abs(ratio9by16-ratio) <= epsilon:
		return "9:16", nil
	default:
		return "other", nil
	}

}
