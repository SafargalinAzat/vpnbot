package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Database struct {
	Conn *sql.DB
}

func NewDatabase(connStr string) (*Database, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	// Ensure new columns exist when upgrading schema
	if _, err := db.Exec(`ALTER TABLE referrals ADD COLUMN IF NOT EXISTS level INT DEFAULT 1`); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	log.Println("Database connected successfully")
	return &Database{Conn: db}, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
            id BIGSERIAL PRIMARY KEY,
            telegram_id BIGINT UNIQUE NOT NULL,
            username TEXT,
            first_name TEXT,
            last_name TEXT,
            created_at TIMESTAMP DEFAULT NOW(),
            marzban_id BIGINT UNIQUE,
            marzban_uuid TEXT UNIQUE,
            is_active BOOLEAN DEFAULT false,
            expire_at TIMESTAMP,
            data_limit BIGINT DEFAULT 0,
            data_used BIGINT DEFAULT 0
        )`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
            id BIGSERIAL PRIMARY KEY,
            user_id BIGINT REFERENCES users(id),
            plan TEXT NOT NULL,
            amount BIGINT NOT NULL,
            status TEXT DEFAULT 'active',
            created_at TIMESTAMP DEFAULT NOW(),
            expire_at TIMESTAMP NOT NULL,
            invoice_id TEXT,
            stars_amount BIGINT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS payments (
            id BIGSERIAL PRIMARY KEY,
            user_id BIGINT REFERENCES users(id),
            subscription_id BIGINT REFERENCES subscriptions(id),
            telegram_id TEXT UNIQUE NOT NULL,
            amount BIGINT NOT NULL,
            currency TEXT DEFAULT 'XTR',
            status TEXT DEFAULT 'pending',
            created_at TIMESTAMP DEFAULT NOW(),
            paid_at TIMESTAMP
        )`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}
	return nil
}

func (db *Database) GetOrCreateUser(telegramID int64, username, firstName, lastName string) (*User, error) {
	var user User
	query := `SELECT id, telegram_id, username, first_name, last_name, created_at, 
                     marzban_id, marzban_uuid, is_active, expire_at, data_limit, data_used 
              FROM users WHERE telegram_id = $1`

	err := db.Conn.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.CreatedAt, &user.MarzbanID, &user.MarzbanUUID, &user.IsActive,
		&user.ExpireAt, &user.DataLimit, &user.DataUsed,
	)

	if err == sql.ErrNoRows {
		// Create new user
		insertQuery := `INSERT INTO users (telegram_id, username, first_name, last_name) 
                        VALUES ($1, $2, $3, $4) RETURNING id`
		err = db.Conn.QueryRow(insertQuery, telegramID, username, firstName, lastName).Scan(&user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		user.TelegramID = telegramID
		user.Username = username
		user.FirstName = firstName
		user.LastName = lastName
		user.CreatedAt = time.Now()
		return &user, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	return &user, nil
}

func (db *Database) CreatePayment(userID int64, amount int64, invoiceID string) (*Payment, error) {
	payment := &Payment{
		UserID:     userID,
		Amount:     amount,
		TelegramID: invoiceID,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	query := `INSERT INTO payments (user_id, amount, telegram_id, status, created_at) 
              VALUES ($1, $2, $3, $4, $5) RETURNING id`

	err := db.Conn.QueryRow(query, payment.UserID, payment.Amount, payment.TelegramID,
		payment.Status, payment.CreatedAt).Scan(&payment.ID)

	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	return payment, nil
}

func (db *Database) ConfirmPayment(invoiceID string) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update payment status
	_, err = tx.Exec(`UPDATE payments SET status = 'paid', paid_at = NOW() 
                      WHERE telegram_id = $1 AND status = 'pending'`, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	return tx.Commit()
}

// Новые методы для работы с реферальной системой
func (db *Database) CreateUserWithReferrer(telegramID int64, username, firstName, lastName string, referrerID sql.NullInt64) (*User, error) {
	// Сначала проверяем, существует ли пользователь
	existingUser, err := db.GetUserByTelegramID(telegramID)
	if err == nil && existingUser != nil {
		// Пользователь уже есть - просто возвращаем его
		return existingUser, nil
	}

	// Если нет - создаем нового
	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	query := `INSERT INTO users (telegram_id, username, first_name, last_name, referrer_id, trial_used, balance, created_at) 
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`

	err = tx.QueryRow(query, telegramID, username, firstName, lastName, referrerID, false, 0, time.Now()).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if referrerID.Valid {
		_, err = tx.Exec(`INSERT INTO referrals (referrer_id, referred_id, created_at) VALUES ($1, $2, $3)`,
			referrerID.Int64, userID, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to create referral record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return db.GetUserByTelegramID(telegramID)
}

func (db *Database) GetUserByTelegramID(telegramID int64) (*User, error) {
	var user User
	query := `SELECT id, telegram_id, username, first_name, last_name, created_at, 
                     marzban_id, marzban_uuid, is_active, expire_at, data_limit, data_used,
                     referrer_id, balance, trial_used, trial_expire_at, last_notified_at
              FROM users WHERE telegram_id = $1`

	err := db.Conn.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.CreatedAt, &user.MarzbanID, &user.MarzbanUUID, &user.IsActive,
		&user.ExpireAt, &user.DataLimit, &user.DataUsed,
		&user.ReferrerID, &user.Balance, &user.TrialUsed, &user.TrialExpireAt,
		&user.LastNotifiedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Пользователь не найден, но это не ошибка
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *Database) GetUserByID(userID int64) (*User, error) {
	var user User
	query := `SELECT id, telegram_id, username, first_name, last_name, created_at, 
                     marzban_id, marzban_uuid, is_active, expire_at, data_limit, data_used,
                     referrer_id, balance, trial_used, trial_expire_at, last_notified_at
              FROM users WHERE id = $1`

	err := db.Conn.QueryRow(query, userID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.CreatedAt, &user.MarzbanID, &user.MarzbanUUID, &user.IsActive,
		&user.ExpireAt, &user.DataLimit, &user.DataUsed,
		&user.ReferrerID, &user.Balance, &user.TrialUsed, &user.TrialExpireAt,
		&user.LastNotifiedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *Database) AddToBalance(userID int64, amount int64) error {
	_, err := db.Conn.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, amount, userID)
	return err
}

func (db *Database) GetReferrals(userID int64) ([]ReferralStat, error) {
	rows, err := db.Conn.Query(`
        SELECT u.id, u.telegram_id, u.username, u.first_name, u.created_at, u.is_active,
               COUNT(p.id) as purchases, COALESCE(SUM(r.commission), 0) as total_commission
        FROM users u
        JOIN referrals r ON r.referred_id = u.id
        LEFT JOIN payments p ON p.user_id = u.id AND p.status = 'paid'
        WHERE r.referrer_id = $1
        GROUP BY u.id
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var referrals []ReferralStat
	for rows.Next() {
		var stat ReferralStat
		err := rows.Scan(&stat.ID, &stat.TelegramID, &stat.Username, &stat.FirstName, &stat.CreatedAt, &stat.IsActive, &stat.Purchases, &stat.TotalCommission)
		if err != nil {
			return nil, err
		}
		referrals = append(referrals, stat)
	}
	return referrals, nil
}

// Пробный период
func (db *Database) ActivateTrial(userID int64, days int) error {
	trialExpire := time.Now().AddDate(0, 0, days)
	_, err := db.Conn.Exec(`
        UPDATE users SET trial_used = true, trial_expire_at = $1 
        WHERE id = $2 AND trial_used = false
    `, trialExpire, userID)
	return err
}

// Уведомления
func (db *Database) GetUsersWithExpiringSubscriptions(daysBefore int) ([]User, error) {
	threshold := time.Now().AddDate(0, 0, daysBefore)

	rows, err := db.Conn.Query(`
        SELECT id, telegram_id, username, first_name, expire_at, last_notified_at
        FROM users 
        WHERE is_active = true 
          AND expire_at IS NOT NULL 
          AND expire_at <= $1
          AND (last_notified_at IS NULL OR last_notified_at < NOW() - INTERVAL '1 day')
    `, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.ExpireAt, &u.LastNotifiedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *Database) UpdateLastNotified(userID int64) error {
	_, err := db.Conn.Exec(`UPDATE users SET last_notified_at = NOW() WHERE id = $1`, userID)
	return err
}

// Начисление комиссии
func (db *Database) AddReferralCommission(referrerID, referredID, amount int64, purchaseID int64) error {
	// Пирамида комиссий (проценты от суммы покупки):
	// 1 уровень (прямой реферал): 30%
	// 2 уровень: 10%
	// 3 уровень: 5%
	// 4 уровень: 2%
	// 5 уровень: 1%
	commissionPercents := []int64{30, 10, 5, 2, 1}

	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentReferrer := referrerID
	for level, pct := range commissionPercents {
		if currentReferrer == 0 {
			break
		}

		commission := amount * pct / 100

		// Обновляем баланс реферера
		_, err = tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, commission, currentReferrer)
		if err != nil {
			return err
		}

		// Записываем в историю (уровень комиссия)
		_, err = tx.Exec(`
        INSERT INTO referrals (referrer_id, referred_id, commission, purchase_id, level, created_at) 
        VALUES ($1, $2, $3, $4, $5, $6)
    `, currentReferrer, referredID, commission, purchaseID, level+1, time.Now())
		if err != nil {
			return err
		}

		// Идем по цепочке вверх: находим реферера текущего реферера
		var nextReferrer sql.NullInt64
		err = tx.QueryRow(`SELECT referrer_id FROM users WHERE id = $1`, currentReferrer).Scan(&nextReferrer)
		if err != nil {
			return err
		}
		if !nextReferrer.Valid {
			break
		}
		currentReferrer = nextReferrer.Int64
	}

	return tx.Commit()
}
