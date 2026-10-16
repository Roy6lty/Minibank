FROM golang:1.27-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/minibank ./cmd

FROM golang:1.27-alpine AS migrator

RUN apk add --no-cache git
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

WORKDIR /app
COPY database/migrations ./database/migrations

ENTRYPOINT ["goose"]

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates \
    && addgroup -S minibank \
    && adduser -S -G minibank minibank

COPY --from=builder --chown=minibank:minibank /out/minibank /minibank

USER minibank
EXPOSE 8080
ENTRYPOINT ["/minibank"]
