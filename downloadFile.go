package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func downloadFile(url string, downloadPath string) (downloadedPath string, err error) {
	// Create base directory
	err = os.MkdirAll(filepath.Dir(downloadPath), 0777)
	if err != nil {
		return "", fmt.Errorf("mkdir error during download file: %w", err)
	}

	// Create the file
	out, err := os.Create(downloadPath)
	if err != nil {
		return "", fmt.Errorf("create file error during download file: %w", err)
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download file error: %w", err)
	} else {
		fmt.Println("Download File:", url)
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("copy file error during download file: %w", err)
	}

	downloadedPath = filepath.Join(downloadPath, filepath.Base(downloadPath))
	return downloadedPath, nil
}
