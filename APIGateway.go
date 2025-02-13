package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	http.HandleFunc("/process", logRequest(processHandler))
	http.HandleFunc("/notify", notifyHandler)

	log.Println("Starting API Gateway on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

// processHandler обрабатывает входящий запрос, выполняет последовательную обработку:
// 1. Принимает изображение через multipart/form-data,
// 2. Отправляет его в Image Service,
// 3. Передаёт результат в генератор SVG,
// 4. Передаёт SVG в генератор шрифта,
// 5. Возвращает сгенерированный шрифт клиенту.
func processHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	file, header, err := r.FormFile("image")
	if err != nil {
		logError(w, "Invalid file", http.StatusBadRequest, start)
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	tee := io.TeeReader(file, &buf)

	logStep("Starting image processing", start)
	notifyEvent("Image processing started")

	imageResp, err := http.Post("http://localhost:8001/process-image", header.Header.Get("Content-Type"), tee)
	if err != nil {
		logError(w, fmt.Sprintf("Image service error: %v", err), http.StatusInternalServerError, start)
		return
	}
	defer imageResp.Body.Close()

	logStep("Image processing completed", start)
	notifyEvent("Image processing completed")

	svgResp, err := http.Post("http://localhost:8002/generate-svg", "application/octet-stream", imageResp.Body)
	if err != nil {
		logError(w, fmt.Sprintf("SVG generation error: %v", err), http.StatusInternalServerError, start)
		return
	}
	defer svgResp.Body.Close()

	logStep("SVG generation completed", start)
	notifyEvent("SVG generation completed")

	fontResp, err := http.Post("http://localhost:8003/generate-font", "application/json", svgResp.Body)
	if err != nil {
		logError(w, fmt.Sprintf("Font generation error: %v", err), http.StatusInternalServerError, start)
		return
	}
	defer fontResp.Body.Close()

	logStep("Font generation completed", start)
	notifyEvent("Font generation completed")

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=font.ttf")
	io.Copy(w, fontResp.Body)

	logSuccess(r, http.StatusOK, start)
	notifyEvent("Request processing completed successfully")
}

// logRequest — middleware для логирования входящих HTTP-запросов.
// Лог выводится в формате:
// [API] 2025/02/10 - 15:04:05 | POST | /process | ::1
//
//	|--> Processing time: 1.0784ms
func logRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[API] %s | %s | %s | %s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
		)
		next(w, r)
		log.Printf("     |--> Processing time: %v", time.Since(start))
	}
}

// logStep выводит информацию о ключевом шаге обработки запроса.
func logStep(message string, start time.Time) {
	log.Printf("     |--> [STEP] %s (elapsed: %v)", message, time.Since(start))
}

// logSuccess выводит финальное сообщение об успешном завершении запроса.
func logSuccess(r *http.Request, status int, start time.Time) {
	log.Printf("[API] %s | %3d | %12v | %15s | %-7s %s",
		time.Now().Format("2006/01/02 - 15:04:05"),
		status,
		time.Since(start),
		getIP(r),
		r.Method,
		r.URL.Path,
	)
	log.Printf("     |--> Request completed successfully")
}

// logError выводит информацию об ошибке и отправляет HTTP-ошибку клиенту.
func logError(w http.ResponseWriter, message string, status int, start time.Time) {
	log.Printf("[API] %s | %3d | %12v | ERROR: %s",
		time.Now().Format("2006/01/02 - 15:04:05"),
		status,
		time.Since(start),
		message,
	)
	http.Error(w, message, status)
}

// getIP возвращает IP-адрес клиента.
func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

// notifyEvent отправляет уведомление (например, о выполнении ключевого шага) на endpoint /notify.
func notifyEvent(message string) {
	payload := map[string]string{"message": message}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal notify payload: %v", err)
		return
	}
	req, err := http.NewRequest("POST", "http://localhost:8000/notify", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create notify request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Type", "site")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send notify request: %v", err)
		return
	}
	defer resp.Body.Close()
}

// notifyHandler принимает уведомления и логирует их.
func notifyHandler(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	log.Printf("[NOTIFY] %v", payload)
	w.WriteHeader(http.StatusOK)
}

/*
Краткие комментарии:
- logRequest — middleware, который логирует входящие HTTP-запросы в формате:
  [API] 2025/02/10 - 15:04:05 | POST | /process | ::1
      |--> Processing time: 1.0784ms
- Функции logStep, logSuccess и logError выводят информацию о ключевых этапах обработки запроса.
- notifyEvent отправляет HTTP POST запрос на endpoint /notify с JSON-сообщением, что имитирует отправку уведомлений боту и клиенту.
- notifyHandler принимает уведомления и логирует их.
- Таким образом, каждый важный этап внешних запросов сопровождается уведомлением.
*/
