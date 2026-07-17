FROM golang:1.26.4-alpine AS builder

WORKDIR /app

COPY app/. ./
RUN go mod download

RUN CGO_ENABLE=0 GOOS=linux go build -ldflags="-w -s" -o job_queue_manager .

FROM alpine:3.19
WORKDIR /root
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/job_queue_manager .
EXPOSE 8080
CMD ["./job_queue_manager"]