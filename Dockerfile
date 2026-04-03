# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Установка зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходников
COPY . .

# Сборка приложения и миграций
RUN go build -o /bin/parser ./cmd/parser
RUN go build -o /bin/migrate ./cmd/migrate

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /bin/parser .
COPY --from=builder /bin/migrate .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./parser"]