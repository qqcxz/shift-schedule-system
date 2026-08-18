# 门店排班系统

店长可按月设置排班，店员可查看排班并提交调休 / 调班 / 改班申请；店长审批通过后，排班表会自动更新，并通过 WebSocket 实时同步到所有在线客户端。

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Stack](https://img.shields.io/badge/stack-Vue3%20%2B%20Go-42b883)](#技术栈)
[![Realtime](https://img.shields.io/badge/realtime-Socket.IO-black)](#实时同步)

---

## 功能特性

- **角色权限**
  - 店长：排班编辑、班次管理、员工管理、申请审批、查看消息
  - 店员：查看排班、提交申请、查看审批结果
- **班次 / 员工管理**
  - 店长可自定义班次名称、时间、颜色
  - 店长可新增员工、修改信息、停用或永久删除账号
- **月度排班**
  - 按月查看全店排班表
  - 店长点击单元格循环切换班次（早班 / 中班 / 晚班 / 休息）
  - 支持批量保存，带版本号防止并发覆盖
  - 支持清理历史排班，可保留最近 N 个月或删除指定月份前数据
- **调休 / 调班 / 改班**
  - 调休：申请将某天改为休息
  - 调班：与指定同事互换当天班次
  - 改班：申请改为指定班次
- **审批流**
  - 店长通过 / 驳回
  - 通过后事务式更新排班
  - 自动通知相关人员
- **虾推啥微信通知**
  - 店长与员工各自绑定个人 Token，店长配置排班发布、排班申请和班前提醒开关
  - Token 加密保存，支持持久化限流、msg_id 留存与投递状态查询
- **实时同步**
  - WebSocket 推送排班变更、申请创建、审批结果、站内消息
- **开箱即用**
  - 本地默认纯 Go SQLite，无需安装 MySQL
  - 首次启动自动初始化演示门店、账号、班次和当月排班

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | Vue 3、Vite、Element Plus、Pinia、Vue Router、Axios、Socket.IO Client |
| 后端 | Go、SQLite/MySQL、JWT、Socket.IO v4 |
| 数据库 | 开发：SQLite；生产：MySQL 8（表结构兼容旧 sql.js 文件） |
| 部署 | Docker 多阶段构建、Docker Compose、Nginx（可选） |

---

## 系统架构

```text
浏览器 / H5 / 微信小程序
  ├─ REST  ──► Go API  ──► SQLite / MySQL
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
│  ├─ server/                 # Go 后端
│  │  ├─ cmd/server/           # Go 程序入口
│  │  ├─ internal/app/         # API、业务、迁移和实时服务
│  │  ├─ go.mod / go.sum
│  │  ├─ Dockerfile
│  │  └─ .env.example
│  ├─ web/                    # Vue3 前端（由 Go 服务托管生产静态文件）
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

- Go 1.24+
- Node.js 22.18+（用于本地构建 Vue 前端）
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

后端默认读取 `apps/server/.env`。Go 服务兼容原来的 `DB_TYPE=sqljs` 配置名，内部使用纯 Go SQLite 驱动。可直接复制示例：

```bash
cp apps/server/.env.example apps/server/.env
```

默认配置：

```env
PORT=3000
DB_TYPE=sqlite
DB_DATABASE=./data/shift.sqlite
DEV_MODE=true
JWT_SECRET=dev-secret-change-me
PUSH_ENCRYPTION_KEY=dev-push-encryption-key-change-me
PUSH_EGRESS_KEY=default-egress
PUSH_RECORD_KEEP_DAYS=30  # 推送记录自动保留天数，超过后每天自动清理
CORS_ORIGIN=http://localhost:5173
SEED_ON_BOOT=true
```

上述配置仅用于本地演示。生产环境必须保持 `DEV_MODE=false`，显式设置强随机 `JWT_SECRET` 和独立的至少 32 字节 `PUSH_ENCRYPTION_KEY`；多实例通过相同 `PUSH_EGRESS_KEY` 共享出口限流。生产默认关闭 seed；如需初始化空库，必须显式提供两个至少 12 位的随机 seed 密码，且所有初始账号首次登录都会强制改密。

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

## 本地开发演示账号

只有使用示例 `.env`（`DEV_MODE=true` 且 `SEED_ON_BOOT=true`）启动空库时才会写入演示数据。生产镜像默认不会创建这些账号，登录页也不会预填或展示账号信息。

| 角色 | 用户名 | 密码 | 说明 |
|------|--------|------|------|
| 店长 | `qqcxz` | `123456` | 可编辑排班、管理班次/员工、审批申请 |
| 店员 | `staff1` | `123456` | 初始店员（可在员工管理中修改/停用） |
| 店员 | `staff2` | `123456` | 初始店员（可在员工管理中修改/停用） |
| 店员 | `staff3` | `123456` | 初始店员（可在员工管理中修改/停用） |

所有演示账号首次登录都会强制设置新密码，密码长度为 6 到 64 位。上述弱密码仅限回环地址上的本机开发，严禁用于公网部署。

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
npm run start:server # 运行本地 Go 后端
```

生产构建后的 Go 服务会托管 `WEB_DIST` 指定的前端静态文件，并通过 `/api` 和 `/socket.io` 提供原有接口。

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
| GET | `/schedules/export?month=YYYY-MM` | 下载当前门店指定月份的 Excel 排班表（店长和店员均可） |
| PUT | `/schedules/month` | 店长批量保存月排班 |
| POST | `/schedules/:month/publish` | 店长发布排班并排队微信通知 |

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

### 虾推啥微信推送

| 方法 | 路径 | 说明 |
|------|------|------|
| GET / PUT | `/push-settings/me` | 查看或绑定本人 Token（只返回掩码） |
| DELETE | `/push-settings/me/token` | 解绑本人 Token |
| POST | `/push-settings/me/test` | 发送测试消息 |
| GET / PATCH | `/stores/current/push-settings` | 查看或由店长修改门店事件开关 |
| GET | `/push-deliveries` | 查看有权限的推送记录 |
| DELETE | `/push-deliveries` | 店长清空本店（含本人和员工），店员清空本人记录 |
| GET | `/push-deliveries/:id/status` | 用已保存的 msg_id 查询投递状态 |

Token 使用 AES-GCM 加密保存。服务持久化执行每 Token 每分钟 10 条、每日 300 条、出口每日 5000 次和状态查询每分钟 5 次限流。发布与业务事件先写入投递队列，由后台 worker 发送，避免第三方网络请求阻塞业务事务。

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
  -e DEV_MODE=false \
  -e DB_TYPE=mysql \
  -e DB_HOST=172.17.0.1 \
  -e DB_PORT=3306 \
  -e DB_USER=shift \
  -e DB_PASSWORD=你的密码 \
  -e DB_NAME=shift_schedule \
  -e JWT_SECRET=请换成强随机字符串 \
  -e PUSH_ENCRYPTION_KEY=请换成独立强随机字符串 \
  -e PUSH_EGRESS_KEY=生产出口标识 \
  -e CORS_ORIGIN=http://你的服务器IP:3000 \
  -e SEED_ON_BOOT=false \
  shift-schedule:latest
```

生产镜像默认 `SEED_ON_BOOT=false`。首次初始化空库如需 seed，请显式设置 `SEED_ON_BOOT=true`、`SEED_MANAGER_PASSWORD` 和 `SEED_STAFF_PASSWORD`；两个初始密码必须分别为至少 12 位的随机值。所有初始账号首次登录都会被强制改密，初始化后建议恢复关闭 seed。

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
- `server`（Go API + Socket.IO + 前端静态资源）

```bash
docker compose up -d --build --remove-orphans
```

`--remove-orphans` 用于清理旧版本的独立 `web` 容器；不要加 `-v`，这样可以保留 MySQL 和 SQLite 数据卷。

Compose 不再默认启动未使用的 Redis，也不会内置数据库密码。请在仓库根目录的未跟踪 `.env` 中至少设置：

```env
MYSQL_ROOT_PASSWORD=替换为强随机密码
MYSQL_PASSWORD=替换为强随机密码
JWT_SECRET=替换为至少 32 字节的强随机字符串
PUSH_ENCRYPTION_KEY=替换为独立的至少 32 字节强随机字符串
PUSH_EGRESS_KEY=生产出口标识
SEED_ON_BOOT=false
```

首次使用空 MySQL 数据库如需初始化演示数据，临时开启 seed，并在未跟踪 `.env` 中设置两个不同的至少 12 位随机初始密码；所有初始账号首次登录都会强制改密。

默认访问：

- 前端：http://服务器IP:3000
- API：http://服务器IP:3000/api
- WebSocket：http://服务器IP:3000/socket.io

### 生产环境变量示例

```env
PORT=3000
DEV_MODE=false
DB_TYPE=mysql
DB_HOST=172.17.0.1
DB_PORT=3306
DB_USER=shift
DB_PASSWORD=你的密码
DB_NAME=shift_schedule
JWT_SECRET=请换成强随机字符串
PUSH_ENCRYPTION_KEY=请换成独立的至少 32 字节强随机字符串
PUSH_EGRESS_KEY=生产出口标识
CORS_ORIGIN=http://你的服务器IP:3000
SEED_ON_BOOT=false
```

建议上线前：

1. 使用至少 32 字节的随机值设置 `JWT_SECRET` 和独立的 `PUSH_ENCRYPTION_KEY`，数据库密码与密钥不要写入仓库
2. 配置 HTTPS（可用宝塔网站反代 / Nginx / Caddy）
3. 生产保持 `SEED_ON_BOOT=false`；若需初始化空库，临时开启并显式提供两个至少 12 位的随机初始密码
4. 定期备份 MySQL

### 容器更新与数据保留

单容器使用 MySQL 时，数据库在容器外，更新镜像不会触碰业务数据。若使用内置 SQLite，请务必挂载 `/app/data`：

```bash
docker volume create shift-schedule-data
docker run -d --name shift-schedule \
  --restart unless-stopped -p 3000:3000 \
  -v shift-schedule-data:/app/data \
  -e DEV_MODE=false -e DB_TYPE=sqlite -e DB_DATABASE=/app/data/shift.sqlite \
  -e JWT_SECRET=请替换为至少32字节随机长字符串 \
  -e PUSH_ENCRYPTION_KEY=请替换为独立的至少32字节随机字符串 \
  -e PUSH_EGRESS_KEY=生产出口标识 -e SEED_ON_BOOT=false \
  ghcr.io/qqcxz/shift-schedule-system:latest
```

升级时先备份 MySQL 或 `/app/data`，再拉取新镜像，停止并删除旧容器后用相同环境变量和挂载重新创建。Go 服务启动时会自动执行幂等迁移并保留已有表数据；不要执行 `docker volume prune`。

从最早版 NestJS 数据库升级时，旧店长用户名可能仍是 `manager`。设置 `SEED_ON_BOOT=true` 时服务会执行一次兼容迁移并将店长带入首次改密流程；如果明确设置为 `false`，则保留旧用户名和密码。

新服务默认使用 `/app/data/shift.sqlite`，并会在新路径为空时识别旧版镜像的 `/app/apps/server/data/shift.sqlite` 文件。

### 方式三：纯手动部署（不用 Docker）

```bash
# 构建前端和 Go 后端
npm install
npm run build

# 启动 Go 后端（会同时托管 apps/web/dist 静态资源）
WEB_DIST=apps/web/dist DB_TYPE=sqlite DB_DATABASE=./data/shift.sqlite npm run start:server

# 也可继续用 Nginx 托管静态资源，并将 /api 与 /socket.io 反代到后端
```

可参考 `deploy/nginx.conf`。
## 业务规则说明

- 只有店长可以正式修改排班
- 店员不能直接改表，只能提交申请
- 审批通过后才会改动正式排班
- 申请当前按单日处理；多日 `toDate` 区间会被明确拒绝，避免只修改首日造成误解
- 跨午夜班次暂不支持，请拆分为相邻日期的班次
- 排班保存使用 `version` 做乐观锁，降低并发覆盖风险
- 同门店数据隔离（按 `storeId`）
- 排班月份和 seed 示例数据使用服务进程的本地时区；当前没有单独的门店时区配置
- 本地 `DEV_MODE=true` 可自动 seed；生产默认并应保持 `SEED_ON_BOOT=false`

---

## 开发说明

### monorepo

前端使用 npm workspace，后端使用独立 Go module：

- `@shift/web`

### 本地数据库

从仓库根目录执行 `npm run dev:server` 时，开发环境默认使用 SQLite，数据库文件路径：

```text
data/shift.sqlite
```

如果直接在 `apps/server` 目录启动 Go 服务，相对路径会以该目录为基准。

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
- 企业微信 / 短信通知
- 微信小程序更完整的实时适配

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
2. 备份并删除当前工作目录下的 `data/shift.sqlite`
3. 重新启动 Go 后端，空库会重新初始化演示数据

---

## License

MIT

---

## 仓库

- GitHub：https://github.com/qqcxz/shift-schedule-system

