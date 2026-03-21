package handlers

import (
	"fmt"
	"log"
	"time"
	"vpnbot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandlers) HandleTrialActivate(query *tgbotapi.CallbackQuery) {

	user, err := h.DB.GetUserByTelegramID(query.From.ID)
	if err != nil {
		h.sendError(query.Message.Chat.ID, "Internal error")
		return
	}

	if user.TrialUsed {
		msg := tgbotapi.NewMessage(query.Message.Chat.ID,
			"❌ Вы уже использовали свой бесплатный пробный период!")
		h.Bot.Send(msg)
		return
	}

	trialDays := 4

	// Создаем пользователя в Marzban с пробным периодом
	username := fmt.Sprintf("trial_%d_%d", user.TelegramID, time.Now().Unix())
	marzbanUser, err := h.Marzban.CreateUser(username, trialDays, 0)
	if err != nil {
		log.Printf("Failed to create trial user: %v", err)
		h.sendError(query.Message.Chat.ID, "Не получилось создать пробную подписку")
		return
	}

	configURL := marzbanUser.SubscriptionURL
	if configURL == "" && len(marzbanUser.Links) > 0 {
		// Если subscription_url пустой, берем первую ссылку из массива
		configURL = marzbanUser.Links[0]
	}

	trialExpire := time.Now().AddDate(0, 0, trialDays)

	tx, err := h.DB.Conn.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		h.sendError(query.Message.Chat.ID, "Internal error")
		return
	}
	defer tx.Rollback()

	// Заблокируем строку пользователя на время транзакции
	var trialUsed bool
	err = tx.QueryRow(`SELECT trial_used FROM users WHERE id = $1 FOR UPDATE`, user.ID).Scan(&trialUsed)
	if err != nil {
		log.Printf("Failed to lock user row: %v", err)
		h.sendError(query.Message.Chat.ID, "Internal error")
		return
	}

	// Еще раз проверяем, что пробный период не использован (может использоваться другим горутином)
	if trialUsed {
		tx.Rollback()
		msg := tgbotapi.NewMessage(query.Message.Chat.ID,
			"❌ Вы уже использовали свой бесплатный пробный период!")
		h.Bot.Send(msg)
		return
	}

	// Обновляем пользователя в той же транзакции
	_, err = tx.Exec(`
        UPDATE users SET 
            trial_used = true,
            trial_expire_at = $1,
            is_active = true,
            expire_at = $1,
            marzban_uuid = $2,
            data_limit = 10
        WHERE id = $3
    `, trialExpire, marzbanUser.Username, user.ID)

	if err != nil {
		log.Printf("Failed to update user: %v", err)
		tx.Rollback()
		h.sendError(query.Message.Chat.ID, "Не удалось сделать запись о пробной подписке")
		return
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		h.sendError(query.Message.Chat.ID, "Не удалось сделать запись о пробной подписке")
		return
	}

	successMsg := fmt.Sprintf(`✅ *Бесплатная пробная версия активирована!*

У вас есть %d дней бесплатного доступа с неограниченным трафиком.

📋 *Ваша подписка:*
`+"`%s`"+`

После окончания пробного периода вы можете приобрести подписку`,
		trialDays, configURL)

	msg := tgbotapi.NewMessage(query.Message.Chat.ID, utils.EscapeMarkdown(successMsg))
	msg.ParseMode = "MarkdownV2"
	h.Bot.Send(msg)

	h.Bot.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
}
