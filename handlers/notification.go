package handlers

import (
	"fmt"
	"time"

	"vpnbot/database"
	"vpnbot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandlers) StartNotificationScheduler() {
	ticker := time.NewTicker(6 * time.Hour) // Проверка каждые 6 часов
	go func() {
		for range ticker.C {
			h.checkExpiringSubscriptions()
		}
	}()
}

func (h *BotHandlers) checkExpiringSubscriptions() {
	// Проверка за 3 дня до истечения
	users3Days, err := h.DB.GetUsersWithExpiringSubscriptions(3)
	if err == nil {
		for _, user := range users3Days {
			h.sendExpirationWarning(&user, 3)
		}
	}

	// Проверка за 1 день до истечения
	users1Day, err := h.DB.GetUsersWithExpiringSubscriptions(1)
	if err == nil {
		for _, user := range users1Day {
			h.sendExpirationWarning(&user, 1)
		}
	}

	// Проверка истекших (вчера)
	expired, err := h.DB.GetUsersWithExpiringSubscriptions(-1)
	if err == nil {
		for _, user := range expired {
			h.sendExpiredNotification(&user)
		}
	}
}

func (h *BotHandlers) sendExpirationWarning(user *database.User, daysLeft int) {
	var text string
	if daysLeft == 1 {
		text = `⚠️ *Ваша подписка истекает ЗАВТРА!*

Не теряйте доступ к VPN.

Продлите подписку сейчас, чтобы продолжить пользоваться сервисом без перебоев.

Используйте /buy для покупки новой подписки`
	} else {
		text = fmt.Sprintf(`ℹ️ *Срок действия вашей подписки истекает через %d дней*

Продлите подписку вовремя, чтобы продолжить пользоваться сервисом без перебоев.

Используйте /buy для покупки новой подписки.`, daysLeft)
	}

	msg := tgbotapi.NewMessage(user.TelegramID, utils.EscapeMarkdown(text))
	msg.ParseMode = "MarkdownV2"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛒 Продлить сейчас", "buy_menu"),
		),
	)
	msg.ReplyMarkup = keyboard

	h.Bot.Send(msg)
	h.DB.UpdateLastNotified(user.ID)
}

func (h *BotHandlers) sendExpiredNotification(user *database.User) {
	text := `❌ *Срок действия вашей подписки истек*

Вы потеряли доступ к VPN.

Продлите подписку сейчас, чтобы продолжить пользоваться сервисом.

Используйте /buy для покупки новой подписки`

	msg := tgbotapi.NewMessage(user.TelegramID, utils.EscapeMarkdown(text))
	msg.ParseMode = "MarkdownV2"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛒 Купить сейчас", "buy_menu"),
		),
	)
	msg.ReplyMarkup = keyboard

	h.Bot.Send(msg)
	h.DB.UpdateLastNotified(user.ID)
}
