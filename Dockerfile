# RkTopNG - one image with the exporter and the dashboard UI inside.
# Build context: the repository root (see docker-compose.yml).

# 1) Frontend: the Svelte app, built to static files.
FROM node:22-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# vite.config.ts writes the build to ../exporter/web/dist
RUN npm run build

# 2) Backend: the Go binary, with the frontend embedded (go:embed).
FROM golang:1.23-alpine AS build
WORKDIR /src/exporter
COPY exporter/ ./
COPY --from=web /src/exporter/web/dist ./web/dist
# go.sum is generated here so the repo can ship with just go.mod.
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /rktop-exporter .

# 3) Runtime - small alpine image. smartmontools provides `smartctl`, used for
# SMART data (disk temperature/health). Runs as root with access to /dev (see
# docker-compose.yml: privileged) so SMART and debugfs (NPU/RGA load) can be read.
FROM alpine:3.21
RUN apk add --no-cache smartmontools
COPY --from=build /rktop-exporter /rktop-exporter
EXPOSE 9888
ENTRYPOINT ["/rktop-exporter"]
