# ---------- build stage ----------
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG BINARY=tenex
RUN case "${BINARY}" in \
        tenex|scheduler|worker) echo "Building binary: ${BINARY}" ;; \
        *) echo "ERROR: invalid BINARY='${BINARY}'. Allowed: tenex, scheduler, worker" && exit 1 ;; \
    esac

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /app/service \
        ./cmd/${BINARY}

# ---------- runtime stage ----------
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/service ./service

ENTRYPOINT ["/app/service"]