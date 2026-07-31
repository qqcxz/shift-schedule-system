FROM golang:1.24-alpine AS builder
WORKDIR /src/apps/server
ARG TARGETOS=linux
ARG TARGETARCH=amd64
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download
COPY apps/server/cmd ./cmd
COPY apps/server/internal ./internal
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/shift-server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
ENV PORT=3000 \
    DB_TYPE=sqlite \
    WEB_DIST=/app/public \
    SEED_ON_BOOT=false
COPY --from=builder /out/shift-server /app/shift-server
RUN mkdir -p /app/data
VOLUME ["/app/data"]
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:3000/api/health || exit 1
ENTRYPOINT ["/app/shift-server"]
