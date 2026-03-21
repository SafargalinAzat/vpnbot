package database

import (
	"database/sql"
	"time"
)

type User struct {
	ID             int64
	TelegramID     int64
	Username       string
	FirstName      string
	LastName       string
	CreatedAt      time.Time
	MarzbanID      sql.NullInt64
	MarzbanUUID    sql.NullString
	IsActive       bool
	ExpireAt       sql.NullTime
	DataLimit      int64
	DataUsed       int64
	ReferrerID     sql.NullInt64
	Balance        int64
	TrialUsed      bool
	TrialExpireAt  sql.NullTime
	LastNotifiedAt sql.NullTime
}

type Subscription struct {
	ID          int64
	UserID      int64
	Plan        string
	Amount      int64
	Status      string
	CreatedAt   time.Time
	ExpireAt    time.Time
	InvoiceID   string
	StarsAmount int64
}

type Payment struct {
	ID             int64
	UserID         int64
	SubscriptionID sql.NullInt64
	TelegramID     string
	Amount         int64
	Currency       string
	Status         string
	CreatedAt      time.Time
	PaidAt         sql.NullTime
}

type Referral struct {
	ID         int64
	ReferrerID int64 // Кто пригласил
	ReferredID int64 // Кого пригласили
	CreatedAt  time.Time
	Commission int64         // Полученная комиссия (в Stars)
	PurchaseID sql.NullInt64 // ID покупки, с которой начислена комиссия
	Level      int           // Уровень (1 — прямой реферал, 2 — реферал реферала и т.д.)
}

type ReferralStat struct {
	User
	Purchases       int
	TotalCommission int64
}

type Notification struct {
	ID       int64
	UserID   int64
	Type     string // trial_expiring, subscription_expiring, subscription_expired
	SentAt   time.Time
	ExpireAt time.Time // Когда истекает (для проверки)
}
