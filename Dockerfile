FROM golang:1.24-alpine AS build

ARG VERSION

# Go module 配置优化
ENV GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn \
    GO111MODULE=on \
    CGO_ENABLED=0

WORKDIR /root

# 先复制依赖文件，利用缓存层（依赖不变时跳过下载）
COPY go.mod go.sum ./

# 禁用 HTTP/2 并下载依赖（解决连接问题）
RUN GODEBUG=http2client=0 go mod download

# 再复制源代码（源码改动不会触发重新下载依赖）
COPY . .

RUN go build --ldflags="-X main.Version=${VERSION}" -o alerthub .

FROM alpine:3.19

COPY --from=build /root/alerthub /app/alerthub

WORKDIR /app

ENTRYPOINT ["/app/alerthub"]