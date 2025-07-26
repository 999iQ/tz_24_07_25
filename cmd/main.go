package main

import (
	"archive/zip"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxUploadSize = 30 * 1024 * 1024 // 30 MB (10MB per file)
	uploadPath    = "./uploads"
	templatesPath = "html/templates"
	maxFiles      = 3
)

func main() {
	if err := os.MkdirAll(uploadPath, os.ModePerm); err != nil {
		log.Fatal("Failed to create upload directory:", err)
	}
	if err := os.MkdirAll(templatesPath, os.ModePerm); err != nil {
		log.Fatal("Failed to create templates directory:", err)
	}

	http.HandleFunc("/upload", uploadFileHandler())
	http.HandleFunc("/", indexHandler())

	fmt.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles(filepath.Join(templatesPath, "index.html"))
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Error executing template: %v", err)
		}
	}
}

func uploadFileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, "Total files size too large (max 30MB)", http.StatusBadRequest)
			return
		}

		files := r.MultipartForm.File["files"]

		if len(files) != maxFiles {
			http.Error(w, fmt.Sprintf("Please upload exactly %d files", maxFiles), http.StatusBadRequest)
			return
		}

		zipPath := filepath.Join(uploadPath, "archive.zip")
		zipFile, err := os.Create(zipPath)
		if err != nil {
			http.Error(w, "Failed to create ZIP archive", http.StatusInternalServerError)
			return
		}
		defer zipFile.Close()
		defer os.Remove(zipPath)

		zipWriter := zip.NewWriter(zipFile)
		defer zipWriter.Close()

		for _, fileHeader := range files {
			ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
			if ext != ".pdf" && ext != ".jpg" && ext != ".jpeg" {
				http.Error(w, "Only PDF and JPEG files are allowed", http.StatusBadRequest)
				return
			}

			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Failed to open uploaded file", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// Создаем запись в архиве
			zipEntry, err := zipWriter.Create(fileHeader.Filename)
			if err != nil {
				http.Error(w, "Failed to create ZIP entry", http.StatusInternalServerError)
				return
			}

			if _, err := io.Copy(zipEntry, file); err != nil {
				http.Error(w, "Failed to write file to ZIP", http.StatusInternalServerError)
				return
			}
		}

		if err := zipWriter.Close(); err != nil {
			http.Error(w, "Failed to finalize ZIP archive", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=archive.zip")
		http.ServeFile(w, r, zipPath)
	}
}
