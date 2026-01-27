FROM golang:1.24-alpine AS build

ARG VERSION

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /root

# 先复制依赖文件，利用缓存层（依赖不变时跳过下载）
COPY go.mod go.sum ./
RUN go mod download

# 再复制源代码（源码改动不会触发重新下载依赖）
COPY . .

RUN CGO_ENABLED=0 go build --ldflags="-X main.Version=${VERSION}" -o alerthub .

FROM alpine:3.19

COPY --from=build /root/alerthub /app/alerthub

WORKDIR /app

ENTRYPOINT ["/app/alerthub"]