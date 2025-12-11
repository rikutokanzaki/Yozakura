FROM golang:1.21-alpine AS builder

RUN apk add --no-cache openssh-keygen

WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o gossh .

FROM alpine:latest

RUN apk add --no-cache openssh-keygen ca-certificates

RUN mkdir -p /certs && \
  ssh-keygen -t rsa -b 2048 -f /certs/ssh_host_rsa_key -N ""

WORKDIR /

COPY --from=builder /gossh .
COPY config/ /config/

RUN mkdir -p /var/log/gossh

ENV HOST_NAME=localhost

CMD ["./gossh"]
