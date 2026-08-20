# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget

RUN adduser -D -H -u 10001 appuser
USER 10001

COPY --from=builder /out/server /usr/local/bin/server

EXPOSE 8080 9090 9100
HEALTHCHECK --interval=5s --timeout=3s --retries=10 CMD wget -qO- http://localhost:9100/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
