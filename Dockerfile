# 单容器部署（适合宝塔 Docker「手动创建」）
# 构建：在项目根目录执行
#   docker build -t shift-schedule:latest .
# 运行：
#   docker run -d --name shift-schedule -p 3000:3000 \
#     -e DB_TYPE=mysql \
#     -e DB_HOST=172.17.0.1 \
#     -e DB_PORT=3306 \
#     -e DB_USER=shift \
#     -e DB_PASSWORD=你的密码 \
#     -e DB_NAME=shift_schedule \
#     -e JWT_SECRET=请改成随机字符串 \
#     -e CORS_ORIGIN=http://你的服务器IP:3000 \
#     -e SEED_ON_BOOT=true \
#     --add-host=host.docker.internal:host-gateway \
#     --restart unless-stopped \
#     shift-schedule:latest

FROM node:22-alpine AS builder
WORKDIR /app

COPY package.json package-lock.json ./
COPY apps/server/package.json apps/server/package.json
COPY apps/web/package.json apps/web/package.json

RUN npm install --include-workspace-root

COPY apps/server apps/server
COPY apps/web apps/web

RUN npm run build -w @shift/web \
  && npm run build -w @shift/server

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
ENV WEB_DIST=/app/public

COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/package-lock.json ./package-lock.json
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/apps/server/package.json ./apps/server/package.json
COPY --from=builder /app/apps/server/dist ./apps/server/dist
COPY --from=builder /app/apps/web/dist ./public

WORKDIR /app/apps/server
EXPOSE 3000
CMD ["node", "dist/main.js"]
