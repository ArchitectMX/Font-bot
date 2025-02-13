package main

import (
	"log"
	"net/http"
	"os/exec"
	"time"
)

// loggingResponseWriter оборачивает http.ResponseWriter и сохраняет статус ответа.
type loggingResponseWriterSVG struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader сохраняет код статуса и вызывает оригинальный WriteHeader.
func (lrw *loggingResponseWriterSVG) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware логирует каждый входящий запрос в формате, похожем на Gin.
func loggingMiddlewareSVG(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriterSVG{ResponseWriter: w, statusCode: http.StatusOK}

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

// generateSVG запускает внешний Python-скрипт для генерации SVG и отправляет результат клиенту.
func generateSVG(w http.ResponseWriter, r *http.Request) {
	buildSVGPath := "build_svg.py"
	cmd := exec.Command("python", buildSVGPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, string(output), http.StatusInternalServerError)
		return
	}

	svgOutputPath := "storage/svgStorage"
	http.ServeFile(w, r, svgOutputPath)
}

func main() {
	http.Handle("/generate-svg", loggingMiddlewareSVG(http.HandlerFunc(generateSVG)))
	log.Println("Сервер запущен на порту 8002")
	log.Fatal(http.ListenAndServe(":8002", nil))
}

/*
Краткие комментарии:
- middleware loggingMiddleware логирует каждый входящий запрос в следующем формате:
  [GIN] 2025/02/10 - 23:17:30 | 200 | 1.0784ms | ::1 | POST "/generate-svg"
- Лог содержит дату, статус ответа, время обработки, IP клиента и метод/путь запроса.
- Функция generateSVG запускает внешний Python-скрипт для генерации SVG и возвращает результат клиенту.
*/
