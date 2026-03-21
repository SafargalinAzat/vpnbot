# Билд-стадия
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Компилируем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -o vpn-bot .

# Финальная стадия
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Копируем бинарник из билд-стадии
COPY --from=builder /app/vpn-bot .
COPY --from=builder /app/.env ./

# Запускаем бота
CMD ["./vpn-bot"]