# ---------- Этап сборки и тестов ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Копируем весь проект (go.mod, main.go, тесты и т.д.)
COPY . .

# Генерируем/обновляем go.sum и подтягиваем зависимости
RUN go mod tidy

# Прогоняем тесты
RUN go test ./...

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -o bot .

# ---------- Этап рантайма ----------
FROM alpine:3.20

# HTTPS для запросов к Telegram API
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Отдельный пользователь без root
RUN adduser -D -g '' botuser && \
    mkdir -p /app/data && \
    chown -R botuser /app

USER botuser

# Только не секретные переменные
ENV DATA_DIR=/app/data

# Копируем собранный бинарник
COPY --from=builder /app/bot /app/bot

# Запускаем бота
CMD ["/app/bot"]