# 门店排班系统

店长可按月设置排班，店员可查看排班并提交调休 / 调班 / 改班申请；店长审批通过后，排班表会自动更新，并通过 WebSocket 实时同步到所有在线客户端。

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Stack](https://img.shields.io/badge/stack-Vue3%20%2B%20NestJS-42b883)](#技术栈)
[![Realtime](https://img.shields.io/badge/realtime-Socket.IO-black)](#实时同步)

---

## 功能特性

- **角色权限**
  - 店长：排班编辑、班次管理、员工管理、申请审批、查看消息
  - 店员：查看排班、提交申请、查看审批结果
- **班次 / 员工管理**
  - 店长可自定义班次名称、时间、颜色
  - 店长可新增员工、修改信息、停用账号
- **月度排班**
  - 按月查看全店排班表
  - 店长点击单元格循环切换班次（早班 / 中班 / 晚班 / 休息）
  - 支持批量保存，带版本号防止并发覆盖
- **调休 / 调班 / 改班**
  - 调休：申请将某天改为休息
  - 调班：与指定同事互换当天班次
  - 改班：申请改为指定班次
- **审批流**
  - 店长通过 / 驳回
  - 通过后事务式更新排班
  - 自动通知相关人员
- **实时同步**
  - WebSocket 推送排班变更、申请创建、审批结果、站内消息
- **开箱即用**
  - 本地默认 `sql.js`，无需安装 MySQL
  - 首次启动自动初始化演示门店、账号、班次和当月排班

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | Vue 3、Vite、Element Plus、Pinia、Vue Router、Axios、Socket.IO Client |
| 后端 | NestJS、TypeORM、JWT、Passport、class-validator、Socket.IO |
| 数据库 | 开发：sql.js；生产：MySQL 8 |
| 部署 | Docker Compose、Nginx |

---

## 系统架构

```text
浏览器 / H5 / 微信小程序
  ├─ REST  ──► NestJS API  ──► sql.js / MySQL
  └─ WS    ──► Socket.IO   ──► 实时事件广播（Web 完整支持，小程序尽力连接 + 刷新兜底）
```

**典型流程：**

1. 店长保存月排班 → 写入数据库 → 广播 `schedule.updated`
2. 店员提交申请 → 写入申请表 → 通知店长 `request.created`
3. 店长审批通过 → 更新申请状态 + 自动改排班 → 广播 `request.resolved` 与 `schedule.updated`

---

## 目录结构

```text
.
├─ apps/
│  ├─ server/                 # NestJS 后端
│  │  ├─ src/
│  │  │  ├─ auth/             # 登录 / JWT
│  │  │  ├─ users/            # 用户
│  │  │  ├─ stores/           # 门店
│  │  │  ├─ shifts/           # 班次模板
│  │  │  ├─ schedules/        # 排班
│  │  │  ├─ requests/         # 调休调班申请与审批
│  │  │  ├─ notifications/    # 消息中心
│  │  │  └─ realtime/         # WebSocket 网关
│  │  ├─ Dockerfile
│  │  └─ .env.example
│  ├─ web/                    # Vue3 前端
│  │  ├─ src/
│  │  │  ├─ api/              # HTTP 接口封装
│  │  │  ├─ views/            # 登录 / 排班 / 班次 / 员工 / 申请 / 消息
│  │  │  ├─ stores/           # Pinia 状态
│  │  │  └─ realtime.ts       # Socket.IO 客户端
│  │  └─ Dockerfile
│  └─ miniprogram/            # 微信小程序原生客户端
│     ├─ pages/               # 登录 / 排班 / 班次 / 员工 / 申请 / 消息
│     ├─ services/            # API 封装
│     └─ utils/               # 鉴权、请求、实时
├─ deploy/
│  ├─ nginx.conf              # 多容器时 Nginx 反代配置
│  └─ BAOTA.md                # 宝塔「手动创建」单容器部署步骤
├─ Dockerfile                 # 单容器镜像（前端 + 后端）
├─ docker-compose.yml         # 可选：多容器部署
├─ package.json               # monorepo workspaces
└─ README.md
```

---

## 环境要求

- Node.js 20+（推荐 22）
- npm 10+
- 生产部署可选：Docker / Docker Compose

---

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/qqcxz/shift-schedule-system.git
cd shift-schedule-system
```

### 2. 安装依赖

```bash
npm install
```

### 3. 配置环境变量（可选）

后端默认读取 `apps/server/.env`。可直接复制示例：

```bash
cp apps/server/.env.example apps/server/.env
```

默认配置：

```env
PORT=3000
DB_TYPE=sqljs
DB_DATABASE=./data/shift.sqlite
JWT_SECRET=dev-secret-change-me
CORS_ORIGIN=http://localhost:5173
SEED_ON_BOOT=true
```

### 4. 启动开发环境

同时启动前后端：

```bash
npm run dev
```

或分别启动：

```bash
npm run dev:server
npm run dev:web
```

### 5. 访问地址

| 服务 | 地址 |
|------|------|
| 前端页面 | http://localhost:5173 |
| 后端入口 | http://localhost:3000/api |
| 健康检查 | http://localhost:3000/api/health |

> 说明：浏览器直接打开 `http://localhost:3000/api` 会返回接口说明 JSON；业务页面请使用前端地址。

---

## 默认账号

首次启动会自动写入初始数据。登录页不再预填或展示账号信息。

| 角色 | 用户名 | 密码 | 说明 |
|------|--------|------|------|
| 店长 | `qqcxz` | `123456` | 可编辑排班、管理班次/员工、审批申请 |
| 店员 | `staff1` | `123456` | 初始店员（可在员工管理中修改/停用） |
| 店员 | `staff2` | `123456` | 初始店员（可在员工管理中修改/停用） |
| 店员 | `staff3` | `123456` | 初始店员（可在员工管理中修改/停用） |

初始化内容包括：

- 示例门店
- 班次模板：早班 / 中班 / 晚班 / 休息
- 当月员工示例排班

---

## 推荐体验路径

1. 使用店长账号 `qqcxz` 登录前端
2. 在「班次管理」调整班次时间，在「员工管理」维护员工账号
3. 在「排班表」点击单元格修改班次，点击保存
4. 另开浏览器隐私窗口，用员工账号登录
5. 在「申请审批」提交调休或调班
6. 回到店长端审批通过，两边排班表和消息中心应实时刷新

---


---

## 微信小程序接入

项目已提供原生小程序客户端：`apps/miniprogram`，与 Web 共用同一后端 API。

### 1. 启动后端

```bash
npm run dev:server
```

### 2. 用微信开发者工具打开

- 导入目录：`apps/miniprogram`
- AppID 可用测试号 / 游客模式
- 开发阶段勾选：**不校验合法域名、web-view（业务域名）、TLS 版本以及 HTTPS 证书**

### 3. 配置后端地址

- 模拟器默认：`http://127.0.0.1:3000`
- 真机调试：在登录页填写电脑局域网 IP，例如 `http://192.168.1.8:3000`

更完整说明见：[`apps/miniprogram/README.md`](./apps/miniprogram/README.md)

默认店长账号：`qqcxz`（密码见上方「默认账号」）；登录页不再展示账号信息。

## 常用脚本

在仓库根目录执行：

```bash
npm run dev          # 同时启动前后端
npm run dev:server   # 仅后端
npm run dev:web      # 仅前端
npm run build        # 构建前后端
npm run start:server # 启动已构建的后端
```

---

## 核心接口

所有接口默认前缀：`/api`

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/auth/login` | 登录，返回 JWT |
| GET | `/auth/me` | 获取当前用户 |

### 基础数据

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/users` | 当前门店用户列表 |
| GET | `/stores/current` | 当前门店信息 |
| GET | `/shifts` | 班次模板列表 |

### 排班

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/schedules?month=YYYY-MM` | 查询月排班 |
| PUT | `/schedules/month` | 店长批量保存月排班 |

### 申请审批

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/requests` | 申请列表（店员仅自己，店长看全店） |
| POST | `/requests` | 提交调休 / 调班 / 改班 |
| POST | `/requests/:id/approve` | 店长通过 |
| POST | `/requests/:id/reject` | 店长驳回 |

### 消息

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/notifications` | 消息列表 |
| POST | `/notifications/:id/read` | 标记单条已读 |
| POST | `/notifications/read-all` | 全部已读 |

### 实时事件（Socket.IO）

| 事件 | 说明 |
|------|------|
| `schedule.updated` | 排班已变更 |
| `request.created` | 新申请 |
| `request.resolved` | 申请已处理 |
| `notification.created` | 新消息 |

连接时需携带登录 JWT：

```js
io('/', {
  path: '/socket.io',
  auth: { token: 'YOUR_JWT' },
  transports: ['websocket'],
})
```

---

## 生产部署

### 方式一：单容器（推荐，适合宝塔「手动创建」）

项目根目录提供 `Dockerfile`，把前端静态资源和后端 API 打进 **同一个镜像**。  
宝塔面板只需创建一个容器，无需容器编排。

```bash
# 1. 构建镜像
docker build -t shift-schedule:latest .

# 2. 运行（数据库用宝塔本机 MySQL 时）
docker run -d \
  --name shift-schedule \
  --restart unless-stopped \
  -p 3000:3000 \
  --add-host=host.docker.internal:host-gateway \
  -e PORT=3000 \
  -e DB_TYPE=mysql \
  -e DB_HOST=172.17.0.1 \
  -e DB_PORT=3306 \
  -e DB_USER=shift \
  -e DB_PASSWORD=你的密码 \
  -e DB_NAME=shift_schedule \
  -e JWT_SECRET=请换成强随机字符串 \
  -e CORS_ORIGIN=http://你的服务器IP:3000 \
  -e SEED_ON_BOOT=true \
  shift-schedule:latest
```

访问：

- 页面：http://服务器IP:3000
- API：http://服务器IP:3000/api
- 健康检查：http://服务器IP:3000/api/health

宝塔逐步说明见：`deploy/BAOTA.md`。

也可直接使用已发布镜像（需仓库 Packages 设为 Public）：

```bash
docker pull ghcr.io/qqcxz/shift-schedule-system:latest
```

宝塔手动创建时，镜像栏填：`ghcr.io/qqcxz/shift-schedule-system:latest`。

### 方式二：Docker Compose（多容器）

项目也提供 `docker-compose.yml`，包含：

- `mysql`
- `redis`（预留）
- `server`（NestJS）
- `web`（Nginx + 前端静态资源）

```bash
docker compose up -d --build
```

默认访问：

- 前端：http://服务器IP
- API：http://服务器IP/api
- WebSocket：http://服务器IP/socket.io

### 生产环境变量示例

```env
PORT=3000
DB_TYPE=mysql
DB_HOST=172.17.0.1
DB_PORT=3306
DB_USER=shift
DB_PASSWORD=你的密码
DB_NAME=shift_schedule
JWT_SECRET=请换成强随机字符串
CORS_ORIGIN=http://你的服务器IP:3000
SEED_ON_BOOT=true
```

建议上线前：

1. 修改默认密码和 `JWT_SECRET`
2. 配置 HTTPS（可用宝塔网站反代 / Nginx / Caddy）
3. 修改默认店长密码，并关闭或限制初始演示店员（`SEED_ON_BOOT=false` 可阻止空库自动种子）
4. 定期备份 MySQL

### 方式三：纯手动部署（不用 Docker）

```bash
# 构建
npm run build

# 启动后端（会同时托管 apps/web/dist 静态资源）
WEB_DIST=apps/web/dist npm run start:server

# 也可继续用 Nginx 托管静态资源，并将 /api 与 /socket.io 反代到后端
```

可参考 `deploy/nginx.conf`。
## 业务规则说明

- 只有店长可以正式修改排班
- 店员不能直接改表，只能提交申请
- 审批通过后才会改动正式排班
- 排班保存使用 `version` 做乐观锁，降低并发覆盖风险
- 同门店数据隔离（按 `storeId`）
- 本地开发默认自动 seed；生产可按需关闭 `SEED_ON_BOOT`

---

## 开发说明

### monorepo

使用 npm workspaces：

- `@shift/server`
- `@shift/web`

### 本地数据库

开发环境默认使用 `sql.js`，数据库文件路径：

```text
apps/server/data/shift.sqlite
```

如需清空演示数据，删除该文件后重启后端即可重新初始化。

### 前端代理

开发模式下，Vite 已代理：

- `/api` → `http://localhost:3000`
- `/socket.io` → `http://localhost:3000`

因此前端代码可直接请求相对路径。

---

## 后续可扩展

- 多门店管理
- 调班需对方先确认，再进入店长审批
- 工时统计 / 加班统计
- Excel 导出
- 企业微信 / 短信通知
- 微信小程序店员端

---

## 常见问题

### 打开 `http://localhost:3000/api` 看到 JSON，这是正常的吗？

是正常的。这是后端 API 服务，不是前端页面。请访问：

- 前端：http://localhost:5173

### 如何确认后端已启动？

访问：

```text
http://localhost:3000/api/health
```

返回类似：

```json
{"ok":true,"service":"shift-schedule-api","time":"..."}
```

### 修改代码后如何重新初始化数据？

1. 停止后端
2. 删除 `apps/server/data/shift.sqlite`
3. 重新启动后端

---

## License

MIT

---

## 仓库

- GitHub：https://github.com/qqcxz/shift-schedule-system

