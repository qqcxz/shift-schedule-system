# 宝塔面板：Docker 手动创建部署指南

目标：只创建一个容器，不用容器编排。  
数据库使用宝塔自带 MySQL。

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

首次构建可能需要几分钟。

## 三、宝塔 Docker → 创建容器 → 手动创建

| 字段 | 填写 |
|------|------|
| 容器名称 | `shift-schedule` |
| 镜像 | `shift-schedule:latest` |
| 端口 | 暴露端口：主机 `3000` → 容器 `3000`（占用则用 `8080:3000`） |
| 重启规则 | 停止后马上重启 |
| 网络 | 默认即可 |
| 挂载 | 可不加 |
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
JWT_SECRET=请改成很长的随机字符串
CORS_ORIGIN=http://你的服务器IP:3000
SEED_ON_BOOT=true
```

如果 `172.17.0.1` 连不上 MySQL，把 `DB_HOST` 改成 `host.docker.internal`，并用命令行加：

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
  -e JWT_SECRET=请改成随机字符串 \
  -e CORS_ORIGIN=http://你的服务器IP:3000 \
  -e SEED_ON_BOOT=true \
  shift-schedule:latest
```

## 五、验证

- 浏览器：`http://服务器IP:3000`
- 健康检查：`http://服务器IP:3000/api/health`

演示账号（`SEED_ON_BOOT=true`）：

| 账号 | 密码 | 角色 |
|------|------|------|
| manager | 123456 | 店长 |
| staff1 | 123456 | 店员 |

## 六、最简模式（不用 MySQL）

环境变量：

```text
PORT=3000
DB_TYPE=sqljs
DB_DATABASE=/app/data/shift.sqlite
JWT_SECRET=随机字符串
CORS_ORIGIN=http://你的服务器IP:3000
SEED_ON_BOOT=true
```

挂载：

```text
主机 /www/wwwroot/shift-data  →  容器 /app/data
```

## 七、绑定域名（可选）

宝塔网站 → 反向代理到 `http://127.0.0.1:3000`  
申请 SSL 后，把 `CORS_ORIGIN` 改成 `https://你的域名` 并重启容器。
