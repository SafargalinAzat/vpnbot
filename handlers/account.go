package handlers

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"vpnbot/utils"
)

func (h *BotHandlers) HandleAccountInfo(chatID int64, telegramID int64) {

	user, err := h.DB.GetOrCreateUser(telegramID, "", "", "")
	if err != nil || !user.MarzbanUUID.Valid {
		msg := tgbotapi.NewMessage(chatID, "❌ Нет активной подписки\n\nИспользуйте /buy для покупки VPN")
		h.Bot.Send(msg)
		return
	}

	if !user.IsActive || !user.ExpireAt.Valid {
		msg := tgbotapi.NewMessage(chatID, "❌ Нет активной подписки\n\nИспользуйте /buy для покупки VPN")
		h.Bot.Send(msg)
		return
	}

	marzbanUser, err := h.Marzban.GetUser(user.MarzbanUUID.String)
	if err != nil || marzbanUser == nil {
		h.sendError(chatID, "Не удалось получилось получить данные о подписке")
		return
	}

	daysLeft := int(time.Until(user.ExpireAt.Time).Hours() / 24)

	// Format data usage
	var dataUsedStr string
	dataUsedStr = fmt.Sprintf("%.2f GB / ∞", float64(marzbanUser.UsedTraffic))

	accountText := fmt.Sprintf(`📊 *Ваш аккаунт*

⏱️ Дней осталось: %d
📊 Трафика использовано: %s

Подписка:

`+"`%s`",
		daysLeft,
		dataUsedStr,
		marzbanUser.SubscriptionURL,
	)

	msg := tgbotapi.NewMessage(chatID, utils.EscapeMarkdown(accountText))
	msg.ParseMode = "MarkdownV2"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Продлить подписку", "buy_menu"),
		),
	)
	msg.ReplyMarkup = keyboard

	h.Bot.Send(msg)
}
