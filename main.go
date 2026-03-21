package main

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"vpnbot/marzban"

	"vpnbot/database"

	"vpnbot/handlers"
)

func main() {
	// Load configuration
	config := LoadConfig()

	// Initialize database
	db, err := database.NewDatabase(config.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Conn.Close()

	// Initialize Marzban client
	marzbanClient := marzban.NewMarzbanClient(
		config.MarzbanURL,
		config.MarzbanUsername,
		config.MarzbanPassword,
	)

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	bot.Debug = false
	log.Printf("Authorized on account @%s", bot.Self.UserName)

	// Initialize handlers
	handlers := &handlers.BotHandlers{
		Bot:      bot,
		DB:       db,
		Marzban:  marzbanClient,
		Prices:   config.Prices,
		AdminIDs: config.AdminIDs,
	}

	// Запускаем планировщик уведомлений
	handlers.StartNotificationScheduler()

	// Set up update channel
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Handle updates asynchronously in worker goroutines
	// This prevents long-running handler operations from blocking the update loop.
	workerSem := make(chan struct{}, 20) // tune the concurrency limit as needed

	for update := range updates {
		workerSem <- struct{}{}
		go func(u tgbotapi.Update) {
			defer func() {
				<-workerSem
				if r := recover(); r != nil {
					log.Printf("panic handling update: %v", r)
				}
			}()

			if u.Message != nil {
				handleMessage(handlers, u.Message)
			} else if u.CallbackQuery != nil {
				handleCallback(handlers, u.CallbackQuery)
			}
		}(update)
	}
}

func handleMessage(h *handlers.BotHandlers, msg *tgbotapi.Message) {
	// Handle commands
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			h.HandleStart(msg)
		case "buy":
			h.HandleBuyMenu(msg.Chat.ID)
		case "account":
			h.HandleAccountInfo(msg.Chat.ID, msg.From.ID)
		}
	}

	// Handle successful payments
	if msg.SuccessfulPayment != nil {
		log.Printf("Received successful payment: %+v", msg.SuccessfulPayment)
		h.HandleSuccessfulPayment(msg)
	}
}

func handleCallback(h *handlers.BotHandlers, query *tgbotapi.CallbackQuery) {
	log.Printf("Received callback: %s from user %d", query.Data, query.From.ID)

	callback := tgbotapi.NewCallback(query.ID, "")
	h.Bot.Send(callback)

	switch query.Data {
	case "buy_menu":
		h.HandleBuyMenu(query.Message.Chat.ID)
	case "account_info":
		h.HandleAccountInfo(query.Message.Chat.ID, query.From.ID)
	case "referrals_info":
		h.HandleReferralsInfo(query.Message.Chat.ID, query.From.ID)
	case "balance_info":
		h.HandleBalanceInfo(query.Message.Chat.ID, query.From.ID)
	case "trial_activate":
		h.HandleTrialActivate(query)
	case "back_to_main":
		h.HandleStart(&tgbotapi.Message{
			Chat: query.Message.Chat,
			From: query.From,
		})
	default:
		if strings.HasPrefix(query.Data, "plan_") {
			plan := strings.TrimPrefix(query.Data, "plan_")
			h.HandlePlanSelection(query, plan)
		}
	}

	// // Удаляем сообщение с кнопками после нажатия (опционально)
	// if query.Data != "back_to_main" && !strings.HasPrefix(query.Data, "plan_") {
	// 	h.Bot.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
	// }
}
