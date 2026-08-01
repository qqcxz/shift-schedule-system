# 宝塔面板：Docker 手动创建部署指南

目标：只创建一个容器，不用容器编排。镜像由 Go 后端、Socket.IO 和 Vue 静态文件组成。
数据库推荐使用宝塔自带 MySQL，也支持挂载数据目录的 SQLite。

## 一、宝塔创建数据库

数据库 → 添加数据库：

- 数据库名：`shift_schedule`
- 用户名：`shift`
- 密码：自定义强密码
- 访问权限：`所有人`（Docker 容器需要）
- 字符集：`utf8mb4`

## 二、服务器准备镜像

SSH / 宝塔终端：

```bash
cd /www/wwwroot
git clone https://github.com/qqcxz/shift-schedule-system.git
cd shift-schedule-system
docker build -t shift-schedule:latest .
```

首次构建会编译 Go 后端和 Vue 前端，可能需要几分钟。

## 三、宝塔 Docker → 创建容器 → 手动创建

| 字段 | 填写 |
|------|------|
| 容器名称 | `shift-schedule` |
| 镜像 | `shift-schedule:latest` |
| 端口 | 暴露端口：主机 `3000` → 容器 `3000`（占用则用 `8080:3000`） |
| 重启规则 | 停止后马上重启 |
| 网络 | 默认即可 |
| 挂载 | MySQL 模式可不加；SQLite 模式必须挂载 `/app/data` |
| command | 留空 |
| entrypoint | 留空 |

环境变量（更多设置里添加）：

```text
PORT=3000
DB_TYPE=mysql
DB_HOST=172.17.0.1
DB_PORT=3306
DB_USER=shift
DB_PASSWORD=你在宝塔创建的数据库密码
DB_NAME=shift_schedule
DEV_MODE=false
JWT_SECRET=至少 32 字节的强随机字符串
PUSH_ENCRYPTION_KEY=独立的至少 32 字节强随机字符串
PUSH_EGRESS_KEY=生产出口标识
CORS_ORIGIN=http://你的服务器IP:3000
SEED_ON_BOOT=false
WEB_DIST=/app/public
```

生产默认关闭 seed。首次部署空库如需初始化演示数据，可临时开启 seed 并显式设置两个不同的强初始密码；所有初始账号首次登录都会强制改密：

```text
SEED_ON_BOOT=true
SEED_MANAGER_USERNAME=qqcxz
SEED_MANAGER_PASSWORD=至少 12 个字符的随机初始密码
SEED_STAFF_PASSWORD=另一个至少 12 个字符的随机初始密码
```

两个 `SEED_*` 变量不是 MySQL 连接密码。生产模式不会接受缺失或弱 seed 密码；初始化完成后建议恢复 `SEED_ON_BOOT=false`。

生产环境推荐 MySQL。若 `172.17.0.1` 连不上 MySQL，把 `DB_HOST` 改成 `host.docker.internal`，并用命令行加：

```bash
--add-host=host.docker.internal:host-gateway
```

## 四、命令行等价创建（可选）

```bash
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
  -e DEV_MODE=false \
  -e JWT_SECRET=至少32字节强随机字符串 \
  -e PUSH_ENCRYPTION_KEY=独立的至少32字节强随机字符串 \
  -e PUSH_EGRESS_KEY=生产出口标识 \
  -e CORS_ORIGIN=http://你的服务器IP:3000 \
  -e SEED_ON_BOOT=false \
  shift-schedule:latest
```

## 五、验证

- 浏览器：`http://服务器IP:3000`
- 健康检查：`http://服务器IP:3000/api/health`

首次 seed 账号（仅在 `users` 表为空且 `SEED_ON_BOOT=true` 时创建）：

| 账号 | 密码 | 角色 |
|------|------|------|
| `SEED_MANAGER_USERNAME` | `SEED_MANAGER_PASSWORD` | 店长 |
| `staff1` | `SEED_STAFF_PASSWORD` | 店员 |

## 六、最简模式（不用 MySQL）

环境变量：

```text
PORT=3000
DB_TYPE=sqlite
DB_DATABASE=/app/data/shift.sqlite
DEV_MODE=false
JWT_SECRET=至少 32 字节的强随机字符串
PUSH_ENCRYPTION_KEY=独立的至少 32 字节强随机字符串
PUSH_EGRESS_KEY=生产出口标识
CORS_ORIGIN=http://你的服务器IP:3000
SEED_ON_BOOT=false
```

SQLite 空库如需 seed，按上文临时开启并设置两个至少 12 位的随机密码；店长和员工首次登录均会强制改密。

挂载（必须）：

```text
主机 /www/wwwroot/shift-data  →  容器 /app/data
```

## 七、更新容器且保留数据

先备份 MySQL 或 SQLite 数据，再执行：

```bash
# 源码部署：拉取代码后重新构建
git pull
docker build -t shift-schedule:latest .
```

若使用已发布镜像，则执行下面的拉取命令，跳过上面的构建：

```bash
docker pull ghcr.io/qqcxz/shift-schedule-system:latest
```

然后停止旧容器并按原配置重新创建：

```bash
docker stop shift-schedule
docker rm shift-schedule
# 使用与原容器完全相同的环境变量、端口和 /app/data 挂载重新创建
```

Go 服务启动时会自动执行幂等数据库迁移；不要删除 MySQL volume 或 SQLite 数据目录，不要执行 `docker volume prune`。

## 八、绑定域名（可选）

宝塔网站 → 反向代理到 `http://127.0.0.1:3000`。申请 SSL 后，把 `CORS_ORIGIN` 改成 `https://你的域名` 并重启容器。API、页面和 Socket.IO 都由同一个 Go 服务处理。


## 九、直接使用已发布镜像（可选）

若 GitHub Actions 已构建并推送镜像，可跳过本机 `docker build`，在宝塔「镜像」栏直接填：

```text
ghcr.io/qqcxz/shift-schedule-system:latest
```

首次公开拉取前，请到 GitHub 仓库 → Packages 将 `shift-schedule-system` 设为 **Public**。

然后容器环境变量仍按上文填写（数据库等配置不变）。
