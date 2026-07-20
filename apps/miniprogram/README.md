# 微信小程序端（门店排班）

本目录是原生微信小程序客户端，直接对接仓库内 NestJS 后端（`apps/server`），与 Web 端共用同一套登录、排班、申请、消息 API。

## 功能

- 登录（JWT）
- 月度排班查看
- 店长点击切换班次并保存
- 调休 / 调班 / 改班申请
- 店长审批
- 消息中心
- 下拉刷新

## 打开方式

1. 先启动后端：

```bash
# 在仓库根目录
npm run dev:server
```

2. 打开微信开发者工具 → **导入项目**
3. 目录选择：

```text
E:\app\apps\miniprogram
```

4. AppID 可先用测试号 / 游客模式（`project.config.json` 默认 `touristappid`）
5. 开发阶段建议：
   - 详情 → 本地设置 → **不校验合法域名、web-view（业务域名）、TLS 版本以及 HTTPS 证书**
6. 编译后使用演示账号登录：
   - `manager / 123456`
   - `staff1 / 123456`

## 后端地址配置

默认：

```text
http://127.0.0.1:3000
```

- 开发者工具模拟器：一般可直接用 `127.0.0.1`
- **真机预览 / 真机调试**：请在登录页把“后端地址”改成电脑局域网 IP，例如：

```text
http://192.168.1.8:3000
```

并确保：

1. 手机和电脑在同一局域网
2. 后端已监听 `0.0.0.0:3000`（已默认开启）
3. 电脑防火墙放行 3000 端口

也可直接改 `app.js`：

```js
globalData: {
  apiBaseUrl: 'http://192.168.1.8:3000',
}
```

## 正式上线注意

1. 后端必须提供 **HTTPS 域名**
2. 在微信公众平台配置 request 合法域名
3. 将 `project.config.json` 中的 `appid` 改成你的小程序 AppID
4. 生产环境关闭“不校验合法域名”

## 目录结构

```text
apps/miniprogram/
├─ app.js / app.json / app.wxss
├─ project.config.json
├─ assets/                 # tabBar 图标
├─ utils/                  # 鉴权、请求、日期、实时
├─ services/api.js         # 接口封装
└─ pages/
   ├─ login/
   ├─ schedule/
   ├─ requests/
   └─ notifications/
```

## 与 Web 的关系

| 能力 | Web (`apps/web`) | 小程序 (`apps/miniprogram`) |
|------|------------------|-----------------------------|
| 登录 | ✅ | ✅ |
| 排班查看/编辑 | ✅ | ✅ |
| 申请审批 | ✅ | ✅ |
| 消息中心 | ✅ | ✅ |
| 实时推送 | Socket.IO Client | 尽力连接 + 页面刷新兜底 |

> 说明：小程序原生 WebSocket 与 Socket.IO 协议不完全兼容。当前已做尽力连接；即使实时通道不可用，页面 `onShow` 与下拉刷新也能保证数据最新。如需完整实时，可后续接入 `weapp.socket.io` 等适配库。
