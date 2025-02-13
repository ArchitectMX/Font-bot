package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// loggingResponseWriter оборачивает http.ResponseWriter для сохранения кода статуса.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader сохраняет код статуса и вызывает исходный метод WriteHeader.
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware оборачивает обработчик и логирует информацию о запросе.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)
		duration := time.Since(start)

		log.Printf("[GIN] %s | %d | %v | %s | %s \"%s\"",
			time.Now().Format("2006/01/02 - 15:04:05"),
			lrw.statusCode,
			duration,
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
		)
	})
}

// processImage обрабатывает запрос, поддерживая multipart и base64, запускает Python-скрипт и возвращает результат.
func processImage(w http.ResponseWriter, r *http.Request) {
	var input io.Reader = r.Body
	if r.Header.Get("Content-Type") == "application/base64" {
		input = base64.NewDecoder(base64.StdEncoding, input)
	}

	tempFile, err := os.CreateTemp("", "image-*.png")
	if err != nil {
		http.Error(w, "Error creating temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())

	if _, err := io.Copy(tempFile, input); err != nil {
		http.Error(w, "Error saving image", http.StatusInternalServerError)
		return
	}
	tempFile.Close()

	scriptPath := "font_grid_extractor.py"
	//cmd := exec.Command("python", scriptPath, tempFile.Name())
	cmd := exec.Command("python", scriptPath, tempFile.Name(), "--dpi", "1", "--rows", "8", "--cols", "9")
	output, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v\nOutput: %s", err, output), http.StatusInternalServerError)
		return
	}

	outputFilePath := ""
	http.ServeFile(w, r, outputFilePath)
}

func main() {
	http.Handle("/process-image", loggingMiddleware(http.HandlerFunc(processImage)))
	log.Println("Сервер запущен на порту 8001")
	log.Fatal(http.ListenAndServe(":8001", nil))
}

/*
Краткие комментарии:
- middleware loggingMiddleware логирует каждый входящий запрос с датой, статусом, временем обработки, IP-адресом и методом/путем запроса.
- Формат лога схож с форматом Gin:
  [GIN] 2025/02/10 - 23:17:30 | 200 | 1.0784ms | ::1 | POST "/process-image"
- processImage поддерживает загрузку файлов в форматах multipart и base64, затем запускает внешний Python-скрипт и возвращает результат клиенту.
*/
