# syntax=docker/dockerfile:1

FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY internal ./internal
COPY pkg ./pkg
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/auren-transfer-agent ./cmd/agent

FROM alpine:3.20
LABEL org.opencontainers.image.title="Auren Transfer Agent"
LABEL org.opencontainers.image.description="High-reliability media transfer agent"
LABEL org.opencontainers.image.version="v1.0.0"
LABEL org.opencontainers.image.source="https://github.com/auren/auren-transfer-agent"
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S auren \
    && adduser -S -G auren -h /var/lib/auren-transfer-agent auren \
    && mkdir -p /etc/auren-transfer-agent /var/lib/auren-transfer-agent/storage /tmp/auren-transfer-agent/downloads \
    && chown -R auren:auren /var/lib/auren-transfer-agent /tmp/auren-transfer-agent
WORKDIR /app
COPY --from=build /out/auren-transfer-agent /usr/local/bin/auren-transfer-agent
COPY --chown=auren:auren configs/agent.yaml /etc/auren-transfer-agent/agent.yaml
ENV AUREN_RUNTIME_ENVIRONMENT=production \
    AUREN_SERVER_ENABLED=true \
    AUREN_SERVER_HOST=0.0.0.0 \
    AUREN_SERVER_PORT=8080 \
    AUREN_RUNTIME_DATA_DIR=/var/lib/auren-transfer-agent \
    AUREN_RUNTIME_TEMP_DIR=/tmp/auren-transfer-agent \
    AUREN_DOWNLOAD_TEMP_DIR=/tmp/auren-transfer-agent/downloads \
    AUREN_STORAGE_LOCAL_PATH=/var/lib/auren-transfer-agent/storage
EXPOSE 8080 9090
USER auren
ENTRYPOINT ["/usr/local/bin/auren-transfer-agent"]
CMD ["--config", "/etc/auren-transfer-agent/agent.yaml"]
