package main

import (
	"log"
	"net/http"
	"os/exec"
	"time"
)

// loggingResponseWriter оборачивает http.ResponseWriter для сохранения кода статуса.
type loggingResponseWriterFont struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeaderFont сохраняет код статуса и вызывает исходный метод WriteHeader.
func (lrw *loggingResponseWriterFont) WriteHeaderFont(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware логирует каждый входящий запрос в формате, похожем на Gin:
func loggingMiddlewareFont(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriterFont{ResponseWriter: w, statusCode: http.StatusOK}

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

// generateFont запускает Python-скрипт для генерации шрифта и возвращает результат клиенту.
func generateFont(w http.ResponseWriter, r *http.Request) {
	buildFontPath := "build_font.py"
	cmd := exec.Command("python", buildFontPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, string(output), http.StatusInternalServerError)
		return
	}

	fontFilePath := "storage/fontStorage/Echoing Ink.ttf"
	http.ServeFile(w, r, fontFilePath)
}

func main() {
	http.Handle("/generate-font", loggingMiddlewareFont(http.HandlerFunc(generateFont)))
	log.Println("Сервер запущен на порту 8003")
	log.Fatal(http.ListenAndServe(":8003", nil))
}

/*
Краткие комментарии:
- middleware loggingMiddleware логирует каждый входящий запрос с датой, статусом ответа,
  временем обработки, IP клиента, HTTP-методом и URL, аналогично логам Gin.
- Формат лога выглядит примерно так:
  [GIN] 2025/02/10 - 23:17:30 | 200 | 1.0784ms | ::1 | POST "/generate-font"
- Функция generateFont запускает внешний Python-скрипт и возвращает сгенерированный файл клиенту.
*/
