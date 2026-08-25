FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app main.go

FROM alpine:latest
RUN apk add --no-cache ffmpeg ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/app ./
COPY --from=builder /app/logo.png ./

EXPOSE 8080
ENV PORT=8080

CMD ["./app"]
