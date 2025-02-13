package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// telegramBot – глобальная переменная для работы с ботом.
var telegramBot *tgbotapi.BotAPI

func main() {
	err := godotenv.Load("config.env")
	if err != nil {
		log.Fatalf("Ошибка загрузки config.env: %v", err)
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("Переменная окружения TELEGRAM_BOT_TOKEN не найдена")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}
	telegramBot = bot

	bot.Debug = true
	log.Printf("Authorized as %s", bot.Self.UserName)

	go func() {
		http.HandleFunc("/notify", notifyHandlerBot)
		log.Println("Notification server started on :9000")
		log.Fatal(http.ListenAndServe(":9000", nil))
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		go handleMessage(bot, update.Message)
	}
}

// handleMessage подготавливает multipart запрос и отправляет файл в API Gateway.
func handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Printf("Получено сообщение от пользователя %d", msg.Chat.ID)

	bodyReader, contentType, err := prepareMultipartRequest(bot, msg)
	if err != nil {
		log.Printf("Request preparation failed: %v", err)
		sendErrorMessage(bot, msg.Chat.ID, "Failed to process your file")
		return
	}

	log.Printf("Отправка запроса в API Gateway на http://localhost:8000/process")
	resp, err := http.Post("http://localhost:8000/process", contentType, bodyReader)
	if err != nil {
		log.Printf("API request failed: %v", err)
		sendErrorMessage(bot, msg.Chat.ID, "Service unavailable")
		return
	}
	defer resp.Body.Close()
	log.Printf("API Gateway вернул статус: %s", resp.Status)

	if resp.StatusCode != http.StatusOK {
		log.Printf("API returned error: %s", resp.Status)
		sendErrorMessage(bot, msg.Chat.ID, "Processing error")
		return
	}

	log.Printf("Отправка шрифта пользователю %d", msg.Chat.ID)
	sendFontFile(bot, msg.Chat.ID, resp.Body)
	log.Printf("Сообщение отправлено пользователю %d", msg.Chat.ID)
}

// prepareMultipartRequest формирует multipart-запрос, загружая файл из Telegram.
func prepareMultipartRequest(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) (io.Reader, string, error) {
	fileID, fileName, err := getFileInfo(msg)
	if err != nil {
		return nil, "", err
	}

	fileURL, err := getFileURL(bot, fileID)
	if err != nil {
		return nil, "", fmt.Errorf("file URL error: %w", err)
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		fileResp, err := http.Get(fileURL)
		if err != nil {
			log.Printf("File download failed: %v", err)
			return
		}
		defer fileResp.Body.Close()

		part, err := writer.CreateFormFile("image", fileName)
		if err != nil {
			log.Printf("Form creation error: %v", err)
			return
		}

		if _, err := io.Copy(part, fileResp.Body); err != nil {
			log.Printf("Data copy error: %v", err)
		}
	}()

	return pr, writer.FormDataContentType(), nil
}

// getFileInfo определяет FileID и FileName в зависимости от типа вложения.
func getFileInfo(msg *tgbotapi.Message) (string, string, error) {
	switch {
	case msg.Document != nil:
		return msg.Document.FileID, msg.Document.FileName, nil
	case len(msg.Photo) > 0:
		photo := msg.Photo[len(msg.Photo)-1]
		return photo.FileID, "image.jpg", nil
	default:
		return "", "", errors.New("unsupported message type")
	}
}

// getFileURL получает URL для скачивания файла из Telegram.
func getFileURL(bot *tgbotapi.BotAPI, fileID string) (string, error) {
	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	tgFile, err := bot.GetFile(fileConfig)
	if err != nil {
		return "", fmt.Errorf("telegram API error: %w", err)
	}
	return tgFile.Link(bot.Token), nil
}

// sendFontFile отправляет сгенерированный файл (шрифт) пользователю.
func sendFontFile(bot *tgbotapi.BotAPI, chatID int64, fontData io.Reader) {
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileReader{
		Name:   "generated_font.ttf",
		Reader: fontData,
	})
	log.Printf("Попытка отправить сгенерированный файл шрифта пользователю %d", chatID)
	if _, err := bot.Send(doc); err != nil {
		log.Printf("Failed to send font: %v", err)
	} else {
		log.Printf("Файл шрифта успешно отправлен пользователю %d", chatID)
	}
}

// sendErrorMessage отправляет сообщение об ошибке пользователю.
func sendErrorMessage(bot *tgbotapi.BotAPI, chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, "❌ "+message)
	log.Printf("Отправка сообщения об ошибке пользователю %d: %s", chatID, message)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// notifyHandler принимает уведомления от API и отправляет их в Telegram.
func notifyHandlerBot(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ChatID  int64  `json:"chat_id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	notifyMsg := tgbotapi.NewMessage(payload.ChatID, payload.Message)
	if _, err := telegramBot.Send(notifyMsg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Notification sent"))
}

/*
Краткие комментарии:
- Этот бот получает файлы от пользователей, отправляет их в API Gateway, получает результат (сгенерированный шрифт) и пересылает его обратно.
- Дополнительно бот запускает HTTP-сервер на порту 9000 с endpoint `/notify`. Когда API отправляет уведомление в формате JSON
  (например, {"chat_id":123456789, "message":"Processing completed"}), бот пересылает это сообщение в указанный чат.
- Таким образом, при изменениях или событиях, происходящих в API, бот может уведомлять пользователя.
*/
