FROM golang:1.25-alpine AS builder

ARG BIN=

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags "-X main.defaultBin=${BIN}" -o /app/server .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .
COPY config.yaml.example config.yaml

EXPOSE 8080

ENTRYPOINT ["./server"]
