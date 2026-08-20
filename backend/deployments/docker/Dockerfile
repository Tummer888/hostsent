FROM golang:1.25-alpine AS builder
WORKDIR /app
# 使用国内镜像加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -o server ./cmd/server/main.go

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server ./server
COPY --from=builder /app/configs ./configs
EXPOSE 8080
CMD ["./server"]
