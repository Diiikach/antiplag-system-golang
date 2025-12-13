package main

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func saveFile(uploadDir, filename string, file multipart.File) error {
	fpath := filepath.Join(uploadDir, filename)
	out, err := os.Create(fpath)
	if err != nil {
		log.Printf("Failed to create file %s: %v", fpath, err)
		return fmt.Errorf("failed to save file")
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		log.Printf("Failed to save file content %s: %v", fpath, err)
		os.Remove(fpath)
		return fmt.Errorf("failed to save file content")
	}

	log.Printf("Successfully uploaded file: %s", filename)
	return nil
}

func serveFile(w http.ResponseWriter, r *http.Request, uploadDir, filename string) error {
	fpath := filepath.Join(uploadDir, filename)
	fileInfo, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found")
	}
	if err != nil {
		log.Printf("Error accessing file %s: %v", fpath, err)
		return fmt.Errorf("internal server error")
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(w, r, fpath)
	log.Printf("Served file: %s (size: %d bytes)", filename, fileInfo.Size())
	return nil
}

func getFilename(urlPath string) string {
	fname := filepath.Base(urlPath)
	if fname == "" || fname == "." || fname == "/" {
		return ""
	}
	return fname
}
