package handlers

import (
	"fmt"
	"vpnbot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandlers) HandleReferralsInfo(chatID int64, telegramID int64) {
	user, err := h.DB.GetUserByTelegramID(telegramID)
	if err != nil {
		h.sendError(chatID, "Internal error")
		return
	}

	referrals, err := h.DB.GetReferrals(user.ID)
	if err != nil {
		h.sendError(chatID, "Failed to get referrals")
		return
	}

	// Подсчет статистики
	totalReferrals := len(referrals)
	var activeReferrals int
	var totalCommission int64

	for _, ref := range referrals {
		if ref.IsActive {
			activeReferrals++
		}
		totalCommission += ref.TotalCommission
	}

	refLink := fmt.Sprintf("https://t.me/%s?start=ref_%d", h.Bot.Self.UserName, user.TelegramID)

	text := fmt.Sprintf(`👥 *Реферальная программа*

🔗 *Ваша реферальная ссылка:*
`+"`%s`"+`

📊 *Статистика:*
• Всего рефералов: %d
• Активных рефералов: %d
• Всего заработано: %d ⭐

💎 *Как это работает:*
• Приглашайте друзей по своей ссылке
• Получайте 30%% от их платежей и 30%% от *любых их вознаграждений* за вновь приглашенных
• Зарабатывайте пассивный доход!

Делитесь своей ссылкой и зарабатывайте!`,
		refLink, totalReferrals, activeReferrals, totalCommission)

	msg := tgbotapi.NewMessage(chatID, utils.EscapeMarkdown(text))
	msg.ParseMode = "MarkdownV2"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", "referrals_info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Back", "back_to_main"),
		),
	)
	msg.ReplyMarkup = keyboard

	h.Bot.Send(msg)
}

func (h *BotHandlers) HandleBalanceInfo(chatID int64, telegramID int64) {
	user, err := h.DB.GetUserByTelegramID(telegramID)
	if err != nil {
		h.sendError(chatID, "Internal error")
		return
	}

	text := fmt.Sprintf(`💰 *Ваш баланс*

Текущий баланс: %d ⭐

*Способы использования баланса:*
• Вывод средств (минимум 5000 ⭐) - свяжитесь с администратором

*Зарабатывайте больше:*
Приглашайте друзей и получайте 30%% от их платежей и 30%% от *любых их вознаграждений* за вновь приглашенных пользователей`,
		user.Balance)

	msg := tgbotapi.NewMessage(chatID, utils.EscapeMarkdown(text))
	msg.ParseMode = "MarkdownV2"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Рефералы", "referrals_info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛒 Купить подписку", "buy_menu"),
		),
	)
	msg.ReplyMarkup = keyboard

	h.Bot.Send(msg)
}
