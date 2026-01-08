FROM golang:1.24-alpine AS build

ARG VERSION

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /root

COPY . /root

RUN CGO_ENABLED=0 go build --ldflags="-X main.Version=${VERSION}" -o alerthub . && \
    chmod +x alerthub

FROM alpine:3.19

COPY --from=build /root/alerthub /app/alerthub

WORKDIR /app

ENTRYPOINT ["/app/alerthub"]