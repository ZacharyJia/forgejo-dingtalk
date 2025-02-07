# 构建阶段
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o forgejo-dingtalk .

# 运行阶段  
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/forgejo-dingtalk /app/
COPY config.example.json /app/config.json

EXPOSE 2525
CMD ["/app/forgejo-dingtalk", "-config", "/app/config.json"]