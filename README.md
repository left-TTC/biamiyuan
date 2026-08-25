# 安全监护产品商城 · 项目总览

一个完整的「安全监护产品商城」全栈项目，包含 **微信小程序用户端**、**Go 服务器**、**React 管理工作台** 三个部分。

- 用户端：微信小程序（安全监护产品商城 + 设备管理 + 会员邀请佣金体系）
- 服务端：Go（HTTP API + SQLite 持久化 + 微信订阅消息）
- 管理端：React + TS + Vite 工作台（数据管理 + Excel 导入导出 + 商品图片上传）

---

## 1. 整体架构

```
┌──────────────────┐       HTTP       ┌────────────────────┐       HTTP       ┌──────────────────┐
│  微信小程序        │◄────────────────►│   Go 服务器          │◄────────────────►│  React 管理工作台  │
│  miniProgram/     │  wx.request      │   server/           │  fetch / 代理     │  admin/           │
│  (用户端)          │                  │   + SQLite app.db   │                  │  (管理端)          │
└──────────────────┘                  │   + uploads/ 图片    │                  └──────────────────┘
                                      └────────────────────┘
                                             ▲
                                             │ 微信订阅消息 subscribeMessage.send
                                      ┌──────┴──────┐
                                      │ 微信服务器     │
                                      └─────────────┘
```

- **小程序**：原生 WXML/WXSS/JS，通过 `utils/api.js` 请求服务器
- **服务器**：统一数据源（商品/分类入库 SQLite），商品图片存 `uploads/` 并静态服务
- **工作台**：管理员登录后管理用户/设备/消息/类目/商品，浏览器端处理 Excel（SheetJS）

---

## 2. 目录结构

```
socialProject/
├── README.md                     # 本文档
├── project.config.json           # 微信开发者工具配置（miniprogramRoot 指向 miniProgram）
├── project.private.config.json   # 工具私有配置（含 urlCheck:false）
│
├── miniProgram/                  # ── 微信小程序（用户端）──
│   ├── app.js                    # 启动：初始化用户、拉取商品缓存、服务器登录同步
│   ├── app.json                  # 页面注册、tabBar（首页/分类/购物车/我的）
│   ├── app.wxss                  # 全局样式（主题色 #1e6fff）
│   ├── components/
│   │   └── navigation-bar/       # 自定义导航栏组件
│   ├── pages/                    # 19 个页面
│   │   ├── index/                # 首页：设备守护状态 + 设备管理入口 + 统一发送警告(测试)
│   │   ├── category/             # 商城（分类 + 商品列表 + 搜索入口）
│   │   ├── cart/ goods/ search/  # 购物车 / 商品详情 / 淘宝风格搜索
│   │   ├── orderConfirm/ orderList/ orderDetail/   # 订单流程
│   │   ├── address/ addressEdit/ # 收货地址
│   │   ├── login/                # 手机号+验证码登录（验证码 12345，微信手机号快捷填充）
│   │   ├── user/                 # 个人中心（会员/余额/退出登录）
│   │   ├── invite/ team/ commission/ withdraw/     # 邀请/团队/佣金/提现
│   │   └── device/ devices/ deviceAdd/             # 设备详情/设备列表/添加设备
│   └── utils/
│       ├── api.js                # 请求封装（BASE_URL 局域网 IP、401 自动重登）
│       ├── store.js              # 本地数据层（用户/购物车/订单/设备/商品缓存）
│       └── util.js               # 时间格式化工具
│
├── server/                       # ── Go 服务器 ──
│   ├── main.go                   # 入口：SQLite、上传目录、微信凭据、管理员密码
│   ├── go.mod                    # module socialserver（含 modernc.org/sqlite）
│   ├── data/app.db               # SQLite 数据库（products/categories/product_edits/sys_info）
│   ├── uploads/                  # 商品图片存储（静态服务 /uploads/*）
│   ├── internal/
│   │   ├── api/
│   │   │   ├── router.go         # 路由注册 + 静态文件服务
│   │   │   ├── auth.go           # 验证码/登录/Bearer 鉴权/wx-login
│   │   │   ├── user.go           # 用户资料接口（昵称/头像上传）
│   │   │   ├── device.go         # 设备与消息接口 + alarm-all
│   │   │   ├── product.go        # 公开商品/分类接口
│   │   │   ├── admin.go          # 管理接口（类目CRUD/商品/批量导入/上传/db-status）
│   │   │   └── debug.go          # DEBUG 模式请求日志中间件
│   │   ├── store/
│   │   │   ├── store.go          # 内存存储 + 用户/设备/消息/admin token
│   │   │   ├── db.go             # SQLite 连接/迁移/审计
│   │   │   ├── catalog.go        # 分类与商品 DB 操作（含种子数据）
│   │   │   └── product.go        # 数据结构定义
│   │   └── wechat/wechat.go      # 微信 API：access_token/code2Session/订阅消息
│   └── README.md                 # 服务器专项文档
│
└── admin/                        # ── React 管理工作台 ──
    ├── package.json              # React18+TS+Vite、react-router、lucide-react、xlsx
    ├── vite.config.ts            # 端口 5173；/api、/uploads 代理到 8080
    ├── index.html
    └── src/
        ├── api.ts                # 请求封装 + 商品 Excel 前端解析/生成
        ├── types.ts              # 数据类型定义
        ├── App.tsx               # 侧边栏布局 + 路由 + 登录守卫
        ├── styles.css            # 全局样式（深蓝科技风）
        ├── main.tsx
        └── pages/
            ├── Login.tsx         # 管理员登录（admin123）
            ├── Dashboard.tsx     # 统计 + 数据库连接验证 + 修改审计
            ├── Users.tsx         # 用户管理
            ├── Devices.tsx       # 设备管理（报警/移除）
            ├── Messages.tsx      # 消息管理
            ├── Categories.tsx    # 类目管理（增删改）
            └── Products.tsx      # 商品管理（编辑/新增/删除/Excel/图片）
```

---

## 3. 技术栈

| 端 | 技术 | 说明 |
|----|------|------|
| 小程序 | 原生 WXML/WXSS/JS | 无第三方框架；`phone-number-quickfill` 官方组件 |
| 服务器 | Go 1.22+ / 标准库 `net/http` | Go 1.22 方法路由 `GET /path/{id}` |
| 数据库 | `modernc.org/sqlite` | 纯 Go 无 CGO，单文件持久化 |
| 工作台 | React 18 + TypeScript + Vite 5 | 类型安全；`react-router-dom` 路由 |
| 工作台图标 | `lucide-react` | 图标库 |
| Excel | `xlsx`（SheetJS） | 浏览器端解析/生成，动态导入拆 chunk |
| 微信能力 | 订阅消息 / code2Session | 需企业主体 + 模板 ID 配置 |

---

## 4. 快速启动（三端）

### 4.1 启动 Go 服务器（必须先启动）
```bash
cd server
go run main.go                 # 默认 :8080
# 可选环境变量：
#   ADDR=:9000                 端口
#   DB_PATH=/data/app.db       数据库路径（默认 data/app.db）
#   UPLOAD_DIR=uploads         图片目录
#   ADMIN_PASSWORD=xxx         管理密码（默认 admin123）
#   WECHAT_APPID / WECHAT_SECRET / WECHAT_TEMPLATE_ID   订阅消息
#   DEBUG=1 或 -debug          请求日志
```

### 4.2 启动管理工作台
```bash
cd admin
npm install
npm run dev                    # http://localhost:5173
# 登录：admin123
```

### 4.3 小程序
1. 微信开发者工具 → 打开项目根目录 `socialProject/`（自动通过 `miniprogramRoot` 定位到 `miniProgram/`）
2. 详情 → 本地设置 → 勾选「不校验合法域名」
3. 真机调试：手机与电脑同一 WiFi；`miniProgram/utils/api.js` 中 `BASE_URL` 需为电脑局域网 IP
   ```bash
   ipconfig getifaddr en0       # macOS 获取电脑局域网 IP
   ```

---

## 5. 核心功能模块

### 5.1 商城
- 商品/分类数据来自服务器 SQLite（`categories`、`products` 表）
- 商品支持**多图**（`images` 数组，第一张为列表头像；详情页 swiper 轮播）
- 商品支持**内置属性**（创建者定义）：如衣服的「尺码 / 颜色」，下单时需选择；普通商品由后台定义，服务商品由团队定义
- 固定**「服务」大类**（写死 `service`，仅后台与团队可发布服务类商品），服务商品标注**服务来源**（团队名/官方）
- 搜索：搜索历史 + 热门词 + 输入联想 + 综合/销量/价格排序（前端本地实现）
- 购物车 → 确认订单 → 支付（真实微信支付：配置商户凭据后拉起 `wx.requestPayment`；未配置时服务器模拟确认）→ 订单状态流转

### 5.2 会员与邀请佣金
- 统一会员身份：登录即会员，所有会员可邀请（无推广员层级）
- 头像/昵称：我的页点击头像或昵称 → 编辑资料页（微信头像昵称填写能力）；头像上传服务器 `uploads/`（失败时本地保存），昵称与头像同步服务器与本地
- 邀请分享：我的页 →「邀请好友」或「我的邀请好友」→ 邀请页点击「分享给微信好友」（`open-type="share"` / 右上角菜单 / 朋友圈），分享路径携带 `?inviter=邀请码`
- 自动绑定：受邀人通过分享链接打开小程序后**注册即自动绑定邀请人**；老用户登录时也会自动绑定（邀请关系持久化到服务器 `users.invited_by`，邀请人可看到被邀请人及消费/佣金统计）
- 手动绑定：邀请页可输入邀请人 6 位邀请码手动绑定（`POST /api/v1/user/bind-inviter`，不能绑定自己，已绑定不可重复）
- 佣金：被邀请人订单支付后生成待结算佣金（订单实付金额 **10%**，`COMMISSION_RATE`），**延迟到无理由退货期（`COMMISSION_SETTLE_DAYS`，默认 7 天）满后自动到账**；退货期内被邀请人退款则佣金自动取消，杜绝「下单即得佣金、退款不回收」
- 佣金明细：`GET /api/v1/user/commissions` 返回待结算 / 已到账 / 已取消记录；邀请页/佣金页展示到账倒计时
- 退款：已支付订单在无理由退货期内可「申请退款」（`POST /api/v1/orders/{id}/refund`），退款即时生效并取消关联待结算佣金
- 提现：真实微信支付模式仅支持提现到微信零钱（后台审核通过后经微信「商家转账」打款）；未配置时本地模拟 + 提现记录

### 5.3 团队与服务发布
- **建团资格**：邀请人数 **> 2** 或所在团队经营金额 **> 1w**（服务器校验，SQLite 持久化 `teams` / `team_members` 表）
- **创建团队**：我的 →「我的团队」→ 满足资格后输入团队名创建；一个用户只能属于一个团队
- **加入团队**：输入团长分享的团队 ID 加入（`POST /api/v1/team/join`），成为团员
- **团员建新团须团长审核**：团员满足建团资格后提交申请（`POST /api/v1/team/apply-create`），现任团长在团队页「待审核建团申请」中通过/驳回；通过后团员自动成为新团团长并从原团队移出
- **发布服务**：团队成员可发布服务类商品（服务大类下二级类目，如安装/看护/定制），**服务来源 = 团队名**；后台也可发布服务（来源默认「官方服务」）
- **经营金额**：团队服务商品成交后按来源团队名累计（`POST /api/v1/team/business`），用于建团资格判定
- **团队金库（服务分成）**：团队服务订单支付成功后，订单金额 **90% 自动入团队金库**、**平台抽成 10%**；经营金额仍按全款累计
- **金库支配**：**仅团长**可提取金库到自己的余额（`POST /api/v1/team/treasury/withdraw`）或从金库直接向团队成员余额转账（`POST /api/v1/team/treasury/transfer`），所有收支记录 `team_treasury_logs` 流水（`GET /api/v1/team/treasury/logs`）
- 团队管理：管理工作台新增「团队管理」页（列表 / 成员 / 经营金额 / 解散）

### 5.3 设备管理
- 首页：设备数量汇总卡 + 统一发送警告测试按钮（本地设备报警 + 服务器订阅推送）
- 设备 CRUD、模拟报警、消息流
- 设备数据目前存**小程序本地**（`mall_devices`），服务器端有对应接口（`/api/v1/devices`）可迁移

### 5.4 微信订阅消息（关闭小程序后收通知）
- 链路：`wx.login` → 服务器 `code2Session` 绑定 openid → 首页「报警微信通知」授权订阅 → 服务器 `alarm-all` 时调 `subscribeMessage.send`
- **需企业主体 + 模板 ID**，配置 `WECHAT_*` 环境变量后生效；未配置时接口返回「模拟推送」

### 5.5 管理工作台
- 仪表盘：数据统计 + **数据库连接验证** + 商品修改审计
- 类目管理（不写死，增删改持久化）
- 商品管理：编辑价格/新增/删除 + **Excel 导入导出（浏览器端 SheetJS）** + **多图上传**
- 用户/设备/消息管理

---

## 6. 数据流

### 商品数据流
```
工作台编辑商品/上传图片 ──► POST /api/v1/admin/products/* ──► SQLite products 表
小程序启动 initProductCache ──► GET /api/v1/products ──► 缓存到本地（含图片完整URL）
小程序商城/详情/购物车 ──► 读取缓存展示
```

### 购物车历史数据流
```
小程序加购/改数/移除 ──► POST /api/v1/cart/sync（全量，记录商品 id 与数量）──► SQLite cart_items 表
小程序登录/冷启动/进入购物车页 ──► GET /api/v1/cart ──► 合并回本地购物车（同商品数量取较大值）
管理工作台 用户管理 ──► GET /api/v1/admin/carts ──► 查看各用户购物车历史
```

### 用户资料数据流
```
我的页 → 编辑资料：chooseAvatar 选头像 ──► POST /api/v1/user/avatar ──► uploads/avatar_*.jpg ──► 保存 URL
昵称输入 type="nickname" ──► PUT /api/v1/user/profile ──► 服务器 User 更新（本地 mall_user 同步）
管理工作台 用户管理 ──► 展示昵称与头像
```

### 登录认证
```
小程序手机号+验证码(12345) ──► 本地登录 + 服务器登录获取 token（存储 mall_server_token）
请求带 Authorization: Bearer token ──► 服务器校验（401 时小程序自动重登）
工作台管理员 admin123 ──► POST /api/v1/admin/login ──► admin token（12h 过期）
```

### 订阅消息推送
```
手机：wx.login code ──► /api/v1/auth/wx-login ──► 服务器存 openid
手机：点「开启报警通知」──► wx.requestSubscribeMessage 授权 ──► /notify/subscribe 记额度
电脑/工作台：触发 alarm-all ──► 服务器 subscribeMessage.send ──► 微信服务通知
```

---

## 7. 数据库（SQLite）

`server/data/app.db`（环境变量 `DB_PATH` 可改），启动自动迁移 + 首次 seed 种子数据：

| 表 | 说明 | 关键列 |
|----|------|--------|
| `categories` | 商品分类（两级 + 服务大类标记） | id, name, parent_id, sort, **is_service** |
| `products` | 商品（`category` 指向**二级类目** ID） | id, name, price, original_price, emoji, colors, **images**, sales, category, tags, detail, **attributes**, **service**, **source_team** |
| `cart_items` | 用户购物车历史 | user_id, product_id, quantity |
| `teams` | 团队 | id, name, owner_phone, owner_name, business_amount |
| `team_members` | 团队成员 | team_id, phone, nick_name, join_time |
| `team_create_requests` | 团员建团申请 | requester_phone, team_name, current_team_id, status(pending/approved/rejected) |
| `product_edits` | 商品修改审计 | product_id, field, old_value, new_value, operator, created_at |
| `sys_info` | 系统元信息 | key, value |

> 用户/设备/消息目前存**服务器内存**（重启清空）；商品/分类/审计/购物车/团队存 SQLite（持久化）。

---

## 8. API 一览

统一响应格式：`{ "code": 0, "msg": "ok", "data": ... }`（`code` 非 0 为错误）

### 公开接口（无需登录）
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /health | 健康检查（含 db 连接状态） |
| GET | /api/v1/categories | 商品分类 |
| GET | /api/v1/products | 商品列表（?category= / ?keyword=） |
| GET | /api/v1/products/{id} | 商品详情 |

### 用户接口（Bearer Token）
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/auth/code | 发验证码（演示固定 12345） |
| POST | /api/v1/auth/login | 手机号+验证码登录 |
| POST | /api/v1/auth/wx-login | wx.login code 绑定 openid |
| GET | /api/v1/user/me | 当前用户 |
| PUT | /api/v1/user/profile | 更新用户资料（昵称 / 头像 URL） |
| POST | /api/v1/user/avatar | 上传头像（multipart，字段 file，保存 uploads/） |
| GET | /api/v1/cart | 当前用户购物车历史（[{productId, quantity}]） |
| POST | /api/v1/cart/sync | 全量同步购物车（记录商品 id 与数量） |
| PUT | /api/v1/cart/items/{productId} | 更新购物车商品数量 |
| DELETE | /api/v1/cart/items/{productId} | 移除购物车商品 |
| GET/POST/DELETE | /api/v1/devices(/{id}) | 设备管理 |
| POST | /api/v1/devices/{id}/alarm | 设备报警 |
| POST | /api/v1/devices/alarm-all | 全部设备警告 + 订阅推送 |
| GET/POST | /api/v1/messages(/{id}/read) | 设备消息 |
| GET/POST | /api/v1/notify/* | 订阅消息模板/授权 |
| GET | /api/v1/team/my | 我的团队（队长/成员） |
| POST | /api/v1/team/create | 创建团队（邀请>2人或经营>1w，须不在任何团队） |
| POST | /api/v1/team/join | 加入团队（输入团队 ID 成为成员） |
| POST | /api/v1/team/apply-create | 团员申请建新团（需现任团长审核） |
| GET | /api/v1/team/requests/my | 我提交的建团申请 |
| GET | /api/v1/team/requests/inbox | 待我审核的建团申请（团长收件箱） |
| POST | /api/v1/team/requests/{id}/approve | 团长审核通过（自动建团并移出原团） |
| POST | /api/v1/team/requests/{id}/reject | 团长审核驳回 |
| POST | /api/v1/team/products | 团队发布服务类商品（服务来源=团队名） |
| POST | /api/v1/team/business | 服务成交累计团队经营金额 |

### 管理接口（admin token，12h 过期）
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/login | 管理员登录（admin123） |
| GET | /api/v1/admin/stats | 统计 |
| GET | /api/v1/admin/db-status | 数据库状态 + 审计 |
| GET | /api/v1/admin/users / devices / messages | 列表 |
| GET | /api/v1/admin/carts | 用户购物车历史 |
| GET | /api/v1/admin/teams / DELETE /{id} | 团队管理 |
| GET | /api/v1/admin/carts | 所有用户购物车历史 |
| POST/DELETE | /api/v1/admin/devices/{id}/alarm / devices/{id} | 设备操作 |
| GET/POST/PUT/DELETE | /api/v1/admin/categories(/{id}) | 类目 CRUD |
| GET/POST | /api/v1/admin/products(/{id}) | 商品增删改查 |
| POST | /api/v1/admin/products/batch | 批量导入（JSON） |
| POST | /api/v1/admin/products/upload | 图片上传（multipart） |

---

## 9. 配置项（服务器环境变量）

| 变量 | 默认 | 说明 |
|------|------|------|
| `ADDR` | `:8080` | 监听地址（监听 0.0.0.0 供真机访问） |
| `DB_PATH` | `data/app.db` | SQLite 路径 |
| `UPLOAD_DIR` | `uploads` | 商品图片目录 |
| `ADMIN_PASSWORD` | `admin123` | 管理密码 |
| `DEBUG` / `-debug` | - | 请求日志模式 |
| `WECHAT_APPID` | - | 微信 AppID（订阅消息/支付必需） |
| `WECHAT_SECRET` | - | 微信 AppSecret |
| `WECHAT_TEMPLATE_ID` | - | 订阅消息模板 ID |
| `WECHAT_PAY_MCH_ID` | - | 微信支付商户号（支付/提现真实模式必需） |
| `WECHAT_PAY_MCH_KEY` | - | 微信支付 APIv3 密钥（32 位） |
| `WECHAT_PAY_SERIAL_NO` | - | 商户 API 证书序列号 |
| `WECHAT_PAY_PRIVATE_KEY_PATH` | - | 商户 API 私钥 `apiclient_key.pem` 路径 |
| `WECHAT_PAY_PUBLIC_KEY_PATH` | - | 微信支付公钥 PEM 路径（转账加密/回调验签） |
| `WECHAT_PAY_BANK_CARD` | - | 企业银行卡号（模拟模式提现打款收款账户） |
| `WECHAT_PAY_NOTIFY_URL` | - | 支付结果回调地址（公网 HTTPS，路径 `/api/v1/pay/notify`） |

---

## 10. 开发指南（如何修改/扩展）

### 新增一个小程序页面
1. 创建 `miniProgram/pages/xxx/xxx.{js,json,wxml,wxss}`（4 文件，4 空格缩进）
2. `miniProgram/app.json` 的 `pages` 数组注册
3. 需要导航栏时在 json 注册 `navigation-bar` 组件，wxml 首行使用

### 新增一个服务器接口
1. `server/internal/api/` 中编写 handler（返回 `ok(w, data)` / `fail(w, code, msg)`）
2. `server/internal/api/router.go` 注册路由（`mux.HandleFunc("GET /path/{id}", ...)`）
3. 数据持久化 → 在 `server/internal/store/` 对应文件加方法（DB 操作参照 `catalog.go`）

### 新增一个工作台页面
1. `admin/src/pages/Xxx.tsx`（参考现有页面：`api.xxx()` 拉数据 + 表格/卡片渲染）
2. `admin/src/App.tsx` 导航 + 路由注册
3. 需要图标用 `lucide-react`；样式追加到 `styles.css`

### 常用修改点速查
| 想改什么 | 去哪改 |
|---------|--------|
| 佣金比例 | `miniProgram/utils/store.js` → `COMMISSION_RATE` |
| 演示验证码 | `miniProgram/utils/store.js` → `SMS_CODE` |
| 服务器地址 | `miniProgram/utils/api.js` → `BASE_URL` |
| 小程序主题色 | `miniProgram/app.wxss`（工作台在 `admin/src/styles.css`） |
| 种子商品/分类 | `server/internal/store/catalog.go` → `seedProducts/seedCategories` |
| 管理员密码 | 服务器环境变量 `ADMIN_PASSWORD` |
| 订阅模板字段 | `server/internal/api/device.go` → `pushAlarmNotify` 的 data 映射 |
| 购物车历史 | 已同步服务器 SQLite（`cart_items` 表，记录 id 与数量） | 可增加合并策略/多端实时同步 |
| 订单/设备存储 | 小程序本地（`mall_orders` / `mall_devices`） | 统一走服务器接口 |

---

## 11. 已知限制与演进方向

| 项 | 现状 | 演进建议 |
|----|------|---------|
| 用户/设备/消息 | 服务器内存存储，重启清空 | 迁移到 SQLite/MySQL |
| 小程序设备 | 存小程序本地，与服务器设备未打通 | 统一走 `/api/v1/devices` |
| 小程序登录 | 本地验证码 12345 | 接服务器 `/auth/login` + 真实短信 |
| 支付/提现 | 未配置商户凭据时模拟确认 | 配置 `WECHAT_PAY_*` 后自动走微信支付 APIv3（`wx.requestPayment` + 回调 + 转账到零钱） |
| 订阅消息 | 需企业主体 + 模板配置 | 配置 `WECHAT_*` 后生效 |
| 商品图片 | 本地 `uploads/` | 生产用对象存储（COS/OSS） |
| Excel 处理 | 浏览器端 SheetJS | 数据量巨大时改服务器端 |
| 工作台认证 | 单管理员密码 | 可扩展多管理员/角色/权限 |

