-- Пользователи
CREATE TABLE IF NOT EXISTS users (
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
    data_used BIGINT DEFAULT 0,
    referrer_id BIGINT REFERENCES users(id),
    balance BIGINT DEFAULT 0,
    trial_used BOOLEAN DEFAULT false,
    trial_expire_at TIMESTAMP,
    last_notified_at TIMESTAMP
);

-- Подписки
CREATE TABLE IF NOT EXISTS subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    plan TEXT NOT NULL,
    amount BIGINT NOT NULL,
    status TEXT DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW(),
    expire_at TIMESTAMP NOT NULL,
    invoice_id TEXT,
    stars_amount BIGINT NOT NULL
);

-- Платежи
CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    subscription_id BIGINT REFERENCES subscriptions(id),
    telegram_id TEXT UNIQUE NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT DEFAULT 'XTR',
    status TEXT DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW(),
    paid_at TIMESTAMP
);

-- Рефералы
CREATE TABLE IF NOT EXISTS referrals (
    id BIGSERIAL PRIMARY KEY,
    referrer_id BIGINT NOT NULL REFERENCES users(id),
    referred_id BIGINT NOT NULL REFERENCES users(id),
    commission BIGINT DEFAULT 0,
    purchase_id BIGINT,
    level INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Уведомления
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    sent_at TIMESTAMP DEFAULT NOW(),
    expire_at TIMESTAMP NOT NULL
);

-- Индексы для скорости
CREATE INDEX idx_users_telegram_id ON users(telegram_id);
CREATE INDEX idx_users_referrer_id ON users(referrer_id);
CREATE INDEX idx_users_expire_at ON users(expire_at) WHERE is_active = true;