FROM golang:1.21-alpine as builder
WORKDIR /app
COPY go.mod .
COPY *.go .
RUN go build -o server .
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
RUN mkdir -p /files/reports
EXPOSE 8002
ENTRYPOINT ["/app/server"]
