FROM golang:1.21-alpine as builder
WORKDIR /app
COPY go.mod .
COPY main.go .
RUN go build -o server main.go
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8000
ENTRYPOINT ["/app/server"]
