# Build Stage
FROM golang:alpine AS builder
WORKDIR /app

RUN apk add --no-cache git
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main ./cmd/api/main.go

# Production Stage
FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Makassar

COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080
CMD ["./main"]
