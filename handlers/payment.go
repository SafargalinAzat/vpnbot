package handlers

import (
	"fmt"
	"log"
	"time"
	"vpnbot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

func (h *BotHandlers) HandleBuyMenu(chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1 месяц - 100 ⭐", "plan_1month"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("3 месяца - 250 ⭐", "plan_3months"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("6 месяцев - 450 ⭐", "plan_6months"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1 год - 850 ⭐", "plan_1year"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "back_to_main"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Выберите тариф:")
	msg.ReplyMarkup = keyboard
	h.Bot.Send(msg)
}

func (h *BotHandlers) HandlePlanSelection(query *tgbotapi.CallbackQuery, plan string) {

	starsAmount := h.Prices[plan]

	// Map plan to days
	var days int
	var title string
	switch plan {
	case "1month":
		days = 30
		title = "VPN Subscription - 1 Month"
	case "3months":
		days = 90
		title = "VPN Subscription - 3 Months"
	case "6months":
		days = 180
		title = "VPN Subscription - 6 Months"
	case "1year":
		days = 365
		title = "VPN Subscription - 1 Year"
	default:
		days = 1
		title = "VPN Subscription"
	}

	// Create invoice
	invoiceID := uuid.New().String()[:8]

	// Save payment to database
	user, err := h.DB.GetUserByTelegramID(query.From.ID)
	if err != nil {
		log.Printf("Error getting user for plan selection: %v", err)
		h.sendError(query.Message.Chat.ID, "Internal error")
		return
	}

	_, err = h.DB.CreatePayment(user.ID, starsAmount, invoiceID)
	if err != nil {
		log.Printf("Error creating payment record: %v", err)
		h.sendError(query.Message.Chat.ID, "Failed to create payment")
		return
	}

	invoice := tgbotapi.NewInvoice(
		query.Message.Chat.ID,
		title,
		fmt.Sprintf("Доступ к VPN %d дней", days),
		invoiceID,
		"",    // provider_token (пусто для Stars)
		"",    // start_parameter (пусто для Stars)
		"XTR", // currency (Stars)
		nil,   // prices (не нужны для Stars)
	)

	// Устанавливаем количество Stars
	invoice.Prices = []tgbotapi.LabeledPrice{
		{
			Label:  title,
			Amount: int(starsAmount), // Количество Stars
		},
	}

	// Avoid sending "null" for suggested_tip_amounts (Telegram expects an array)
	invoice.SuggestedTipAmounts = []int{}

	// Отправляем инвойс
	_, err = h.Bot.Send(invoice)
	if err != nil {
		log.Printf("Error sending invoice: %v", err)
		h.sendError(query.Message.Chat.ID, "Не получилось создать инвойс")
		return
	}
}

func (h *BotHandlers) notifyReferralChain(referredID, amount int64) {
	percents := []int64{30, 10, 5, 2, 1}
	user, err := h.DB.GetUserByID(referredID)
	if err != nil || user == nil {
		return
	}

	currentRef := user.ReferrerID
	for i, pct := range percents {
		if !currentRef.Valid {
			return
		}

		referer, err := h.DB.GetUserByID(currentRef.Int64)
		if err != nil || referer == nil {
			return
		}

		commission := amount * pct / 100
		if commission <= 0 {
			currentRef = referer.ReferrerID
			continue
		}

		text := fmt.Sprintf("🎉 *Уровень %d*: вы получили %d ⭐ от покупки в вашей цепочке", i+1, commission)
		msg := tgbotapi.NewMessage(referer.TelegramID, utils.EscapeMarkdown(text))
		msg.ParseMode = "MarkdownV2"
		h.Bot.Send(msg)

		currentRef = referer.ReferrerID
	}
}

func (h *BotHandlers) HandleSuccessfulPayment(message *tgbotapi.Message) {

	payment := message.SuccessfulPayment

	user, err := h.DB.GetUserByTelegramID(message.From.ID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		h.sendError(message.Chat.ID, "Internal error")
		return
	}

	// Сохраняем платеж
	paymentRecord, err := h.DB.CreatePayment(user.ID, int64(payment.TotalAmount), payment.InvoicePayload)
	if err != nil {
		h.sendError(message.Chat.ID, "Internal error")
		return
	}

	// Создаем пользователя в Marzban
	username := fmt.Sprintf("user_%d_%d", user.TelegramID, time.Now().Unix())
	days := h.getDaysFromAmount(int64(payment.TotalAmount))

	marzbanUser, err := h.Marzban.CreateUser(username, days, 0)
	if err != nil {
		h.sendError(message.Chat.ID, "Payment successful but failed to create VPN user")
		return
	}

	// НАЧИСЛЯЕМ КОМИССИЮ РЕФЕРЕРУ (30%)
	if user.ReferrerID.Valid {
		err = h.DB.AddReferralCommission(
			user.ReferrerID.Int64,
			user.ID,
			int64(payment.TotalAmount),
			paymentRecord.ID,
		)
		if err != nil {
			log.Printf("Failed to add referral commission: %v", err)
		} else {
			// Уведомляем всю цепочку рефералов
			h.notifyReferralChain(user.ID, int64(payment.TotalAmount))
		}
	}

	// Сообщение пользователю
	successMsg := fmt.Sprintf(`✅ *Оплата прошла успешно!*\n\n`+
		`Ваша VPN подписка активна на %d дней.\n\n`+
		`📋 *Подписка:*\n`+
		"`%s`\n\n"+
		`💰 Баланс: %d ⭐\n`+
		`Используйте /account для просмотра деталей`,
		days, marzbanUser.SubscriptionURL, user.Balance)

	msg := tgbotapi.NewMessage(message.Chat.ID, utils.EscapeMarkdown(successMsg))
	msg.ParseMode = "MarkdownV2"
	h.Bot.Send(msg)
}

func (h *BotHandlers) getDaysFromAmount(amount int64) int {
	switch amount {
	case 100:
		return 30
	case 250:
		return 90
	case 450:
		return 180
	case 850:
		return 365
	default:
		return 30
	}
}
