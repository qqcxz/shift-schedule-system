# 单容器部署：Go API、Socket.IO 和 Vue 静态页面
# 最终镜像不包含 Node.js、npm、Go 工具链或源码。

FROM node:22-alpine AS web-builder
WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web/package.json apps/web/package.json
RUN npm ci --workspace=@shift/web --include-workspace-root
COPY apps/web apps/web
RUN npm run build -w @shift/web

FROM golang:1.24-alpine AS server-builder
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
    WEB_DIST=/app/public \
    DB_TYPE=sqlite \
    SEED_ON_BOOT=false
COPY --from=server-builder /out/shift-server /app/shift-server
COPY --from=web-builder /src/apps/web/dist /app/public
RUN mkdir -p /app/data
VOLUME ["/app/data"]
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:3000/api/health || exit 1
ENTRYPOINT ["/app/shift-server"]
