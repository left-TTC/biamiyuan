# 安全监护商城 · Go 服务端

安全监护产品商城的后端服务，与 `miniProgram/`（微信小程序端）配套。

## 技术栈

- Go 1.22+（标准库 `net/http`，无第三方依赖）
- 内存存储（演示环境；生产可替换为 MySQL/Redis）

## 快速启动

```bash
cd server
go run main.go
# 或指定端口 / 数据库路径
ADDR=:9000 DB_PATH=/data/app.db go run main.go
# 或启用 DEBUG 模式（输出请求/响应日志）
go run main.go -debug
DEBUG=1 go run main.go
```

## 数据库

内置 **SQLite**（`modernc.org/sqlite`，纯 Go 无 CGO）：

- 默认数据库文件 `data/app.db`（环境变量 `DB_PATH` 可改）
- 启动时自动建表并持久化**商品修改审计**（`product_edits`）
- 工作台仪表盘可查看数据库连接状态（`GET /api/v1/admin/db-status`）
- 商品修改接口写入审计日志（字段、旧值→新值、操作人、时间），重启不丢失

DEBUG 模式示例输出：

```text
2026/08/17 15:58:58 [debug] POST /api/v1/auth/login
    req : {"phone":"test","code":"12345"}
    resp: 200 {"code":0,"msg":"ok","data":{"token":"...","user":{...}}}
 (137µs)
```

启动后服务监听 `http://localhost:8080`，健康检查：`GET /health`。

## 接口一览

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | /health | 健康检查 | 否 |
| POST | /api/v1/auth/code | 发送验证码（演示固定 `12345`） | 否 |
| POST | /api/v1/auth/login | 手机号+验证码登录，未注册自动创建会员 | 否 |
| GET | /api/v1/user/me | 当前用户信息 | Bearer Token |
| GET | /api/v1/categories | 商品分类列表 | 否 |
| GET | /api/v1/products | 商品列表（`?category=`/`?keyword=` 筛选） | 否 |
| GET | /api/v1/products/{id} | 商品详情 | 否 |
| GET | /api/v1/devices | 我的设备列表 | Bearer Token |
| POST | /api/v1/devices | 添加设备 | Bearer Token |
| DELETE | /api/v1/devices/{id} | 移除设备 | Bearer Token |
| POST | /api/v1/devices/{id}/alarm | 单设备报警上报（预留订阅消息推送） | Bearer Token |
| POST | /api/v1/devices/alarm-all | 向所有设备发送警告 + 微信订阅消息推送 | Bearer Token |
| GET | /api/v1/messages | 设备消息列表 | Bearer Token |
| POST | /api/v1/messages/{id}/read | 标记消息已读 | Bearer Token |
| POST | /api/v1/auth/wx-login | 用 wx.login code 绑定微信 openid | Bearer Token |
| GET | /api/v1/notify/template-id | 订阅消息模板 ID | Bearer Token |
| POST | /api/v1/notify/subscribe | 记录订阅授权（一次性订阅额度 +1） | Bearer Token |

鉴权方式：请求头 `Authorization: Bearer <token>`（登录接口返回）。

## 快速体验

```bash
# 1. 获取验证码
curl -X POST localhost:8080/api/v1/auth/code -d '{"phone":"test"}'

# 2. 登录
curl -X POST localhost:8080/api/v1/auth/login -d '{"phone":"test","code":"12345"}'
# 返回 { "token": "...", "user": {...} }

# 3. 添加设备（携带 token）
curl -X POST localhost:8080/api/v1/devices \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"客厅烟雾报警器","type":"smoke","sn":"SMK1234"}'

# 4. 模拟设备报警
curl -X POST localhost:8080/api/v1/devices/<设备id>/alarm \
  -H "Authorization: Bearer <token>"
```

## 对接小程序端

小程序 `miniProgram/` 已接入本服务：

- **商品/分类**：`utils/api.js` 请求 `GET /api/v1/categories`、`GET /api/v1/products`，
  `utils/store.js` 启动时拉取并缓存（`initProductCache`），分类/详情/搜索页均读缓存
- **登录**：`pages/login/login.js` 当前为演示（本地验证码 `12345`），
  可接入 `POST /api/v1/auth/code` + `POST /api/v1/auth/login` 换取 token
- **设备/消息**：首页设备管理目前为本地存储演示，可替换为 `/api/v1/devices`、`/api/v1/messages`

> 开发工具调试：详情 → 本地设置 → 勾选「不校验合法域名」；
> 真机调试：将 `miniProgram/utils/api.js` 的 `BASE_URL` 改为电脑局域网 IP。

## 微信订阅消息（退出小程序后仍可收通知）

实现「手机在关闭小程序后，电脑点击发送仍能收到微信通知」需要满足微信平台要求：

1. **企业主体小程序**（个人主体无法使用订阅消息）
2. 微信公众平台 → 功能 → **订阅消息** → 申请模板（如"设备安全报警"），获得模板 ID
3. 服务器配置环境变量启动：

```bash
export WECHAT_APPID=你的AppID
export WECHAT_SECRET=你的AppSecret
export WECHAT_TEMPLATE_ID=你的模板ID
go run main.go
```

4. 完整链路（已实现）：
   - 手机打开小程序 → 登录后自动 `wx-login` 绑定 openid
   - 首页「报警微信通知」开关 → `wx.requestSubscribeMessage` 授权订阅 → `notify/subscribe` 记录额度
   - **电脑端**点首页「统一发送警告」→ `POST /api/v1/devices/alarm-all` → 服务端调 `subscribeMessage.send`
   - 手机即使**关闭小程序**，也能在微信「服务通知」收到报警消息

5. 未配置 `WECHAT_*` 时：推送为**模拟模式**（接口正常返回，服务器日志可见，界面提示"模拟推送"）

> 注意：订阅消息为**一次性订阅**，每次推送需用户再次授权（微信平台机制）；
> 模板字段（thing1/time2）需与申请模板的字段对应，见 `internal/wechat/wechat.go`。

## 局域网真机调试

```bash
# 1. 启动服务器（监听 0.0.0.0:8080）
go run main.go

# 2. 查看电脑局域网 IP（手机与电脑需同一 WiFi）
ipconfig getifaddr en0   # macOS

# 3. 若 IP 与小程序端配置不同，修改
#    miniProgram/utils/api.js 中 BASE_URL = 'http://<你的IP>:8080'
```

- 手机微信扫码进入开发者工具的「真机调试」即可访问
- 需在开发者工具「详情-本地设置」勾选「不校验合法域名」
- 若无法连接，检查 macOS 防火墙是否允许 8080 端口
