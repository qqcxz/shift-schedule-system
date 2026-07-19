# 排班表系统

店长排班 + 店员查看/调休调班申请 + 实时审批更新。

## 技术栈

- 前端：Vue 3 + Vite + Element Plus + Pinia + Socket.IO Client
- 后端：NestJS + TypeORM + JWT + Socket.IO
- 数据库：默认 sql.js（本地零配置），生产可切换 MySQL
- 部署：Docker Compose + Nginx

## 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 启动开发环境

```bash
npm run dev
```

- 前端：http://localhost:5173
- 后端：http://localhost:3000
- 健康检查：http://localhost:3000/api/health

也可分别启动：

```bash
npm run dev:server
npm run dev:web
```

### 3. 演示账号

| 角色 | 用户名 | 密码 |
|------|--------|------|
| 店长 | manager | 123456 |
| 店员 | staff1 | 123456 |
| 店员 | staff2 | 123456 |
| 店员 | staff3 | 123456 |

首次启动会自动初始化门店、班次模板、当月示例排班。

## 功能

- 店长按月设置/修改排班（点击单元格切换班次后保存）
- 店员查看个人与全店排班
- 店员提交调休 / 调班 / 改班申请
- 店长审批通过后自动更新排班
- WebSocket 实时推送排班与申请变更
- 消息中心查看通知

## 生产部署（MySQL）

1. 修改环境变量（可直接改 `docker-compose.yml` 或 `apps/server/.env`）：

```env
DB_TYPE=mysql
DB_HOST=mysql
DB_PORT=3306
DB_USER=shift
DB_PASSWORD=shift123
DB_NAME=shift_schedule
JWT_SECRET=请换成强随机字符串
CORS_ORIGIN=https://your-domain.com
```

2. 启动：

```bash
docker compose up -d --build
```

访问：http://服务器IP

## 目录结构

```text
apps/
  server/   NestJS API + WebSocket
  web/      Vue3 管理端/店员端
deploy/     Nginx 配置
docker-compose.yml
```

## 核心接口

- `POST /api/auth/login`
- `GET /api/schedules?month=YYYY-MM`
- `PUT /api/schedules/month`
- `POST /api/requests`
- `POST /api/requests/:id/approve`
- `POST /api/requests/:id/reject`
- `GET /api/notifications`
