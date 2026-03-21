package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"vpnbot/database"
	"vpnbot/marzban"
	"vpnbot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotHandlers struct {
	Bot      *tgbotapi.BotAPI
	DB       *database.Database
	Marzban  *marzban.MarzbanClient
	Prices   map[string]int64
	AdminIDs []int64
}

func (h *BotHandlers) HandleStart(message *tgbotapi.Message) {
	// Проверяем, есть ли реферsальный код
	var referrerID sql.NullInt64
	if len(message.CommandArguments()) > 0 {
		// Формат: ref_123456789
		parts := strings.Split(message.CommandArguments(), "_")
		if len(parts) == 2 && parts[0] == "ref" {
			if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				// Проверяем, что реферер существует
				if referrer, _ := h.DB.GetUserByTelegramID(id); referrer != nil {
					referrerID = sql.NullInt64{Int64: referrer.ID, Valid: true}
				}
			}
		}
	}

	user, err := h.DB.CreateUserWithReferrer(
		message.From.ID,
		message.From.UserName,
		message.From.FirstName,
		message.From.LastName,
		referrerID,
	)
	if err != nil {
		h.sendError(message.Chat.ID, "Internal error")
		return
	}

	// Предлагаем пробный период, если не использован
	var statusText string
	if !user.TrialUsed {
		statusText = "🎁 Вам доступна бесплатная пробная подписка!"
	} else if user.IsActive && user.ExpireAt.Valid && user.ExpireAt.Time.After(time.Now()) {
		daysLeft := int(time.Until(user.ExpireAt.Time).Hours() / 24)
		statusText = fmt.Sprintf("✅ Подписка активна\n📅 Осталось дней: %d", daysLeft)
	} else {
		statusText = "❌ У вас нет активной подписки"
	}

	// Реферальная ссылка
	refLink := fmt.Sprintf("https://t.me/%s?start=ref_%d", h.Bot.Self.UserName, user.TelegramID)

	refferalText := "Приглашайте друзей и получайте 30%% от их платежей и 30%% от *любых их вознаграждений* за вновь приглашенных пользователей"

	msg := tgbotapi.NewMessage(message.Chat.ID,
		utils.EscapeMarkdown(fmt.Sprintf("Добро пожаловать в Narod VPN!\n\n%s\n\n"+refferalText+"\n\n💰 Ваш баланс: %d ⭐\n🔗 Ваша реферальная ссылка: `%s`",
			statusText, user.Balance, refLink)))

	msg.ParseMode = "MarkdownV2"
	keyboard := h.getMainKeyboard(user)
	msg.ReplyMarkup = keyboard

	h.Bot.Send(msg)
}

func (h *BotHandlers) getMainKeyboard(user *database.User) tgbotapi.InlineKeyboardMarkup {
	var buttons [][]tgbotapi.InlineKeyboardButton

	// Кнопка пробного периода, если доступен
	if !user.TrialUsed {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎁 Пробная подписка", "trial_activate"),
		))
	}

	buttons = append(buttons,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛒 Купить VPN", "buy_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Аккаунт", "account_info"),
			tgbotapi.NewInlineKeyboardButtonData("👥 Рефералы", "referrals_info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Баланс", "balance_info"),
			tgbotapi.NewInlineKeyboardButtonURL("ℹ️ Инструкция", "https://telegra.ph/Vse-nashi-gajdiki-12-27"),
		),
	)

	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
}

func (h *BotHandlers) sendError(chatID int64, errMsg string) {
	msg := tgbotapi.NewMessage(chatID, "❌ Ошибка: "+errMsg+"\n\nСообщите @ о проблеме")
	h.Bot.Send(msg)
}
