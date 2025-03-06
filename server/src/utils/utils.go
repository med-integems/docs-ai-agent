package utils

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kkdai/youtube/v2"
)

// MaxFileSize defines the maximum allowable file size in bytes (1 GB)
const MaxFileSize = 1 * 1024 * 1024 * 1024 // 1 GB
func DownloadVideo2(videoURL, path string) (string, error) {
	// Create a YouTube client
	client := youtube.Client{}
	// Fetch video details
	video, err := client.GetVideo(videoURL)
	if err != nil {
		return "", fmt.Errorf("failed to get video details: %w", err)
	}

	fmt.Printf("Downloading Video: %s\n", video.Title)

	// Get the best available format
	streamInfo := video.Formats[1]

	// fmt.Printf("Video format %v", streamInfo)
	stream, _, err := client.GetStream(video, &streamInfo)
	if err != nil {
		return "", fmt.Errorf("failed to get video stream: %w", err)
	}

	log.Println("URI", streamInfo.URL)

	// Check for video size limit
	videoSize, err := GetVideoSizebyURI(streamInfo.URL)
	if err != nil {
		return "", fmt.Errorf("failed to get video size: %w", err)
	}

	if videoSize > MaxFileSize {
		return "", errors.New("video size exceeds the 1 GB limit")
	}

	fmt.Printf("Video Size: %.2f MB\n Quality %v\n", videoSize, streamInfo.Quality)

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Create a file to save the video
	filePath := filepath.Join(cwd, "downloads", path)
	file, err := os.Create(filePath)

	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Stream data directly into the file
	if _, err := io.Copy(file, stream); err != nil {
		return "", fmt.Errorf("failed to save video: %w", err)
	}

	// Resize video
	// ResizeVideo(filePath, 500, 300)
	fmt.Println("Video downloaded successfully")
	return video.Title, nil
}

func DownloadVideo(videoURL, fileName string) (string, error) {
	client := youtube.Client{}

	video, err := client.GetVideo(videoURL)
	if err != nil {
		return "", fmt.Errorf("failed to get video: %w", err)
	}

	// Select the best quality format
	formats := video.Formats.WithAudioChannels() // Get formats with audio
	if len(formats) == 0 {
		return "", fmt.Errorf("no suitable video formats found")
	}

	// Sort by quality and pick the best one
	formats.Sort()
	streamInfo := formats[1]
	// log.Println("URI", streamInfo.URL)

	stream, size, err := client.GetStream(video, &streamInfo)
	if err != nil {
		log.Println("Error", err)
		return "", fmt.Errorf("failed to get stream: %w", err)
	}

	// log.Println("Stream Data", stream)
	log.Print("Size =>", size)

	defer stream.Close()

	if size == 0 {
		return "", fmt.Errorf("invalid video size")
	}

	videoSize, err := GetVideoSizebyURI(streamInfo.URL)
	if err != nil {
		return "", fmt.Errorf("failed to get video size: %w", err)
	}

	if videoSize > MaxFileSize {
		return "", errors.New("video size exceeds the 1 GB limit")
	}

	fmt.Printf("Title: %v\n\rVideo Size: %.2f MB\n\rQuality %v\n", video.Title, videoSize, streamInfo.Quality)

	// Create a file to save the video

	if err := os.MkdirAll("downloads", 0755); err != nil {
		return "", fmt.Errorf("failed to create downloads directory: %w", err)
	}

	// Create a file to save the video
	filePath := filepath.Join("downloads", fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	defer file.Close()
	// log.Println(stream)
	// Stream with progress tracking
	written, err := io.Copy(file, stream)
	if err != nil {
		fmt.Println("Error", err.Error())
		if err2 := os.Remove(filePath); err2 != nil {
			log.Println("Error removing file", err2.Error())
		}
		return "", fmt.Errorf("failed to save video: %w", err)
	}

	if written == 0 {
		os.Remove(filePath)
		return "", fmt.Errorf("zero bytes written")
	}

	fmt.Printf("Video downloaded successfully: %.2f MB\n", float64(written)/1024*1024)
	return video.Title, nil
}

// func ResizeVideo(inputPath string, width, height int) error {
// 	first, err := moviego.Load(inputPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to resize video: %w", err)
// 	}
// 	err = first.Resize(int64(width), int64(height)).Output("file.mp4").Run()
// 	if err != nil {
// 		return fmt.Errorf("failed to resize video: %w", err)
// 	}
// 	// Check for video size limit
// 	// videoSize, err := GetVideoSize(inputPath)
// 	// if err != nil {
// 	// 	return fmt.Errorf("failed to get video size: %v", err)
// 	// }
// 	log.Printf("Video resized successfully! \n\r New Size : %v", "")
// 	return nil
// }

// GetVideoSize takes a file path and returns its size in MB
func GetVideoSize(filePath string) (float64, error) {
	// Get file information
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0.0, fmt.Errorf("failed to get file size: %w", err)
	}

	// Calculate size in MB
	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	return fileSizeMB, nil
}

func GetVideoSizebyURI(streamURL string) (float64, error) {
	resp, err := http.Head(streamURL)
	if err != nil {
		log.Println("Error", err)
		return 0.0, fmt.Errorf("failed to get video size: %w", err)
	}
	defer resp.Body.Close()
	log.Println(float64(resp.ContentLength))
	return float64(resp.ContentLength) / 1024 / 1024, nil // Convert bytes to MB
}
