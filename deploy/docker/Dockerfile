# ---------- build ----------
FROM golang:1.25-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o tenex ./cmd/bot
RUN CGO_ENABLED=0 GOOS=linux go build -o scheduler ./cmd/scheduler
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

# ---------- runtime ----------
FROM alpine:3.19

WORKDIR /app
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/tenex .
COPY --from=builder /app/scheduler .
COPY --from=builder /app/worker .

CMD ["./tenex"]