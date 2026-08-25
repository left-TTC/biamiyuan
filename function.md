# 安全监护产品商城 · 软件功能表

> 依据项目根目录 `README.md` 整理，反映当前系统实现的功能清单。
> 系统由三端组成：**微信小程序用户端（miniProgram/）**、**Go 服务器（server/）**、**React 管理工作台（admin/）**。

---

## 一、项目概述

一个完整的「安全监护产品商城」全栈项目：

- **用户端**：微信小程序（安全监护产品商城 + 设备管理 + 会员邀请佣金体系）
- **服务端**：Go（HTTP API + SQLite 持久化 + 微信订阅消息）
- **管理端**：React + TS + Vite 工作台（数据管理 + Excel 导入导出 + 商品图片上传）

---

## 二、总体架构

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

## 三、技术栈一览

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

## 四、快速启动（三端）

| 端 | 启动方式 | 访问/说明 |
|----|---------|----------|
| Go 服务器 | `cd server && go run main.go` | 默认 `:8080`；可选 `ADDR / DB_PATH / UPLOAD_DIR / ADMIN_PASSWORD / WECHAT_* / DEBUG` 环境变量 |
| 管理工作台 | `cd admin && npm install && npm run dev` | `http://localhost:5173`；登录密码 `admin123` |
| 小程序 | 微信开发者工具打开项目根目录 `socialProject/`（自动定位 `miniProgram/`） | 详情→本地设置→勾选「不校验合法域名」；真机调试需把 `utils/api.js` 的 `BASE_URL` 改为电脑局域网 IP（`ipconfig getifaddr en0`） |

---

## 五、微信小程序用户端功能

| 序号 | 功能模块 | 功能项 | 功能说明 | 代码位置 |
|----|---------|--------|---------|---------|
| 1 | 启动初始化 | 用户初始化 | 启动时初始化用户身份 | `app.js` |
| 2 | 启动初始化 | 商品缓存 | 启动时拉取商品并缓存到本地 | `app.js` + `utils/store.js` |
| 3 | 启动初始化 | 服务器登录同步 | 启动时与服务端登录同步 | `app.js` |
| 4 | 首页 | 设备守护状态 | 设备数量汇总卡 + 守护状态展示 | `pages/index/` |
| 5 | 首页 | 设备管理入口 | 提供设备管理页面入口 | `pages/index/` |
| 6 | 首页 | 统一发送警告（测试） | 一键触发全部设备报警 + 服务器订阅推送 | `pages/index/` |
| 7 | 首页 | 报警微信通知授权 | 「报警微信通知」订阅授权（`wx.requestSubscribeMessage`） | `pages/index/` |
| 8 | 商城 | 分类展示 | 按分类浏览商品 | `pages/category/` |
| 9 | 商城 | 商品列表 | 分类内商品列表展示 | `pages/category/` |
| 10 | 商城 | 搜索入口 | 跳转搜索页 | `pages/category/` |
| 11 | 商品详情 | 多图轮播 | 商品多图 `swiper` 轮播，第一张为列表头像 | `pages/goods/` |
| 12 | 商品详情 | 内置属性 | 创建者定义的商品属性（如尺码/颜色），下单前须选择 | `pages/goods/` + `utils/store.js` |
| 13 | 购物车 | 商品管理 | 加入/增减/删除购物车商品 | `pages/cart/` + `utils/store.js` |
| 14 | 购物车 | 历史同步 | 加购/改数/移除同步服务器 `cart_items`；登录/冷启动/进入购物车页拉取合并 | `utils/store.js` + `api/cart.go` |
| 15 | 订单流程 | 确认订单 | 确认订单信息（地址/商品/备注/金额） | `pages/orderConfirm/` |
| 16 | 订单流程 | 支付 | 微信支付（配置商户凭据后拉起 `wx.requestPayment`，微信回调确认；未配置时服务器模拟） | `pages/orderDetail/` + `utils/store.js` |
| 17 | 订单流程 | 状态流转 | 待付款 → 待发货 → 待收货 → 已完成 / 已取消（服务器持久化） | `pages/orderList/` + `utils/store.js` |
| 18 | 订单流程 | 订单列表/详情 | 按状态查看订单列表与订单详情（含物流信息/联系客服） | `pages/orderList/` + `pages/orderDetail/` |
| 19 | 搜索 | 搜索历史 | 记录并展示历史搜索词 | `pages/search/` |
| 20 | 搜索 | 热门词 | 展示热门搜索词 | `pages/search/` |
| 21 | 搜索 | 输入联想 | 输入时联想建议 | `pages/search/` |
| 22 | 搜索 | 排序 | 综合/销量/价格三种排序（前端本地实现） | `pages/search/` |
| 23 | 收货地址 | 地址列表 | 管理收货地址（绑定账号保存在服务器，默认地址唯一） | `pages/address/` |
| 24 | 收货地址 | 地址编辑 | 新增/编辑收货地址（含"设为默认"开关） | `pages/addressEdit/` |
| 25 | 登录 | 手机号+验证码 | 演示验证码固定 12345 | `pages/login/` + `utils/store.js` |
| 26 | 登录 | 微信手机号快捷填充 | 官方 `phone-number-quickfill` 组件 | `pages/login/` |
| 27 | 登录 | 双端登录 | 本地登录 + 服务器登录取 token（存 `mall_server_token`） | `pages/login/` + `utils/api.js` |
| 28 | 个人中心 | 会员/余额 | 展示会员身份与账户余额 | `pages/user/` |
| 29 | 个人中心 | 编辑资料 | 头像（`chooseAvatar` + 上传服务器/本地兜底）+ 昵称（`type=nickname`），本地保存并同步服务器 | `pages/profile/` + `utils/store.js` |
| 30 | 个人中心 | 退出登录 | 退出当前账号 | `pages/user/` |
| 31 | 邀请 | 邀请码分享 | 「分享给微信好友」（`open-type=share` / 右上角菜单 / 朋友圈），路径携带 `?inviter` 邀请码 | `pages/invite/` |
| 32 | 邀请 | 自动绑定 | 受邀人通过分享链接打开后注册/登录自动绑定邀请人 | `app.js` + `utils/store.js` |
| 33 | 邀请 | 手动绑定 | 邀请页输入 6 位邀请码手动绑定（服务器持久化 `users.invited_by`，不能绑自己、不可重复） | `pages/invite/` + `utils/store.js` + `api/user.go` |
| 34 | 邀请 | 邀请记录 | 查看已邀请好友列表（服务器持久化，含累计消费与佣金统计）+ 本地模拟成员 | `pages/invite/` + `api/user.go` |
| 35 | 团队 | 建团资格 | 邀请人数 > 2 或所在团队经营金额 > 1w 可创建团队（服务器校验） | `pages/team/` + `utils/store.js` |
| 36 | 团队 | 创建/加入团队 | 输入团队名创建；输入团队 ID 加入（`/team/join`），一用户一团队 | `pages/team/` + `api/team.go` |
| 37 | 团队 | 团员建团审核 | 团员申请建新团（`/team/apply-create`），团长收件箱通过/驳回，通过后自动建团并移出原团 | `pages/team/` + `api/team.go` + `store/team.go` |
| 38 | 团队 | 发布服务 | 团队成员发布服务类商品（服务来源=团队名，可定义服务属性），展示在商城「服务」类目 | `pages/team/` + `api/team.go` |
| 38b | 团队 | 团队金库 | 服务订单 90% 分成入金库（平台抽成 10%）；团长提取到余额/向成员转账/金库流水 | `pages/team/` + `utils/store.js` |
| 39 | 佣金 | 佣金记录 | 服务端持久化佣金（订单 10%）；无理由退货期（`COMMISSION_SETTLE_DAYS`）满后到账，退款则取消 | `pages/commission/` + `store/commission.go` |
| 39b | 订单 | 无理由退款 | 已支付订单退货期内申请退款（即时生效），关联待结算佣金自动取消 | `pages/orderDetail/` + `api/order.go` |
| 40 | 提现 | 提现申请 | 微信零钱提现（真实模式经微信「商家转账」打款，收款账户为绑定 openid；未配置时模拟） | `pages/withdraw/` |
| 41 | 提现 | 提现记录 | 查看历史提现记录（后台审核：处理中/已到账/已驳回） | `pages/withdraw/` + `utils/store.js` |
| 41b | 客服 | 会话列表/新建 | 我的客服会话 + 新建会话（自动分配客服） | `pages/support/` |
| 41c | 客服 | 会话聊天 | 消息收发；团队客服成员可回复/关闭分配会话 | `pages/supportChat/` |
| 41d | 客服 | 入口 | 商品详情/订单详情/个人中心"联系客服"入口 | `pages/goods/` + `pages/orderDetail/` + `pages/user/` |
| 42 | 设备 | 设备列表/详情 | 设备列表与详情展示（数据存本地 `mall_devices`） | `pages/devices/` + `pages/device/` |
| 43 | 设备 | 添加设备 | 新增设备 | `pages/deviceAdd/` |
| 44 | 设备 | 设备 CRUD | 设备增删改查 | `pages/devices/` + `utils/store.js` |
| 45 | 设备 | 模拟报警 | 触发单个设备模拟报警 | `pages/device/` |
| 46 | 设备 | 消息流 | 设备报警消息展示 | `pages/device/` |
| 47 | 全局 | 自定义导航栏 | `navigation-bar` 自定义导航栏组件 | `components/navigation-bar/` |
| 48 | 全局 | 请求封装 | `BASE_URL`（局域网 IP）、401 自动重登 | `utils/api.js` |
| 49 | 全局 | 本地数据层 | 用户/购物车/订单/设备/商品缓存 | `utils/store.js` |
| 50 | 全局 | 工具函数 | 时间格式化 | `utils/util.js` |


---

## 六、Go 服务器功能

| 序号 | 功能模块 | 功能项 | 功能说明 | 代码位置 |
|----|---------|--------|---------|---------|
| 1 | 服务入口 | 服务启动 | SQLite 初始化、上传目录、微信凭据、管理员密码配置 | `main.go` |
| 2 | 健康检查 | `/health` | 健康检查（含 db 连接状态） | `internal/api/router.go` |
| 3 | 认证 | 发验证码 | 演示固定验证码 12345 | `internal/api/auth.go` |
| 4 | 认证 | 注册/登录 | 手机号+验证码登录、Bearer Token 签发 | `internal/api/auth.go` |
| 5 | 认证 | wx-login | `wx.login` code 换取 openid 并绑定用户 | `internal/api/auth.go` + `internal/wechat/wechat.go` |
| 6 | 认证 | 鉴权中间件 | `Authorization: Bearer` 校验，401 自动重登 | `internal/api/auth.go` |
| 7 | 用户 | 用户资料 | 更新昵称/头像 URL（`PUT /user/profile`） | `internal/api/user.go` |
| 8 | 用户 | 头像上传 | multipart 上传头像到 `uploads/`（`POST /user/avatar`） | `internal/api/user.go` |
| 9 | 商品 | 分类/商品查询 | 公开接口：分类列表、商品列表（分类/关键词过滤）、商品详情 | `internal/api/product.go` |
| 10 | 购物车 | 历史同步 | 全量同步/查询/改数/删除购物车（`cart_items` 表） | `internal/api/cart.go` + `internal/store/cart.go` |
| 11 | 购物车 | 管理端查看 | 所有用户购物车历史 | `internal/api/cart.go` |
| 12 | 设备 | 设备管理 | 设备 CRUD、模拟报警、alarm-all | `internal/api/device.go` |
| 13 | 消息 | 设备消息 | 设备消息列表与已读 | `internal/api/device.go` |
| 14 | 订阅消息 | 模板/授权 | 模板 ID 查询、订阅授权额度 | `internal/api/device.go` |
| 15 | 订阅消息 | 消息推送 | 触发报警时 `subscribeMessage.send`；未配置时模拟推送 | `internal/wechat/wechat.go` |
| 16 | 团队 | 我的团队 | 队长/成员视角查询（SQLite 持久化） | `internal/api/team.go` + `internal/store/team.go` |
| 17 | 团队 | 创建/加入 | 建团资格校验（邀请>2人或经营>1w）、加入团队 | `internal/api/team.go` |
| 18 | 团队 | 建团申请 | 团员申请、团长收件箱、审核通过/驳回 | `internal/api/team.go` + `internal/store/team.go` |
| 19 | 团队 | 发布服务 | 团队发布服务类商品（校验服务类目 + 团队成员身份） | `internal/api/team.go` |
| 20 | 团队 | 经营金额 | 服务成交后按来源团队累计（建团资格依据） | `internal/api/team.go` + `internal/store/team.go` |
| 20b | 团队 | 金库分成 | 服务订单 90% 入金库 + 10% 平台抽成；团长提取/转账（事务内原子扣减 + 流水） | `internal/store/team.go` + `internal/api/team.go` |
| 21 | 管理端 | 登录 | 管理员登录（admin / staff，bcrypt 校验） | `internal/api/admin.go` |
| 22 | 管理端 | 鉴权 | admin token（12h 过期）+ 角色校验（superAdmin） | `internal/api/admin.go` |
| 23 | 管理端 | 统计 | 用户/设备/消息/商品/类目/报警统计 | `internal/api/admin.go` |
| 24 | 管理端 | 用户/购物车 | 用户列表、购物车历史查看 | `internal/api/admin.go` |
| 25 | 管理端 | 设备/消息 | 设备列表、报警、移除；消息查看/删除 | `internal/api/admin.go` |
| 26 | 管理端 | 团队管理 | 团队列表（含成员/经营金额）、移除团队 | `internal/api/admin.go` |
| 27 | 管理端 | 类目管理 | 类目增删改查（持久化） | `internal/api/admin.go` |
| 28 | 管理端 | 商品管理 | 商品增删改查、批量导入（JSON） | `internal/api/admin.go` |
| 29 | 管理端 | 图片上传 | 商品图片 multipart 上传到 `uploads/` | `internal/api/admin.go` |
| 30 | 管理端 | 账号管理 | admin 创建/删除员工账号、角色/状态管理 | `internal/api/admin.go` + `internal/store/admin_account.go` |
| 30b | 地址 | 地址接口 | 地址 CRUD + 默认地址（绑定账号，服务器 SQLite） | `internal/api/order.go` + `internal/store/order.go` |
| 30c | 订单 | 订单接口 | 创建（服务器计价+快照）/列表/详情/取消/支付/确认收货 | `internal/api/order.go` + `internal/store/order.go` |
| 30d | 订单 | 发货接口 | 管理端发货（绑定物流公司与单号） | `internal/api/order.go` + `internal/store/order.go` |
| 30e | 提现 | 提现接口 | 申请（真实模式绑定 openid 走微信转账）/记录/审核（done 微信打款 / failed 自动退款） | `internal/api/order.go` + `internal/store/order.go` |
| 30g | 支付 | 微信支付 | 微信支付 APIv3 客户端：JSAPI 下单 / 回调验签解密 / 退款 / 转账到零钱（未配置回退模拟） | `internal/wechatpay/wechatpay.go` |
| 30f | 客服 | 客服接口 | 开单自动分配、消息收发、团队客服收件箱/回复/关闭、管理端收件箱 | `internal/api/support.go` + `internal/store/support.go` |
| 30g | 团队 | 指定客服成员 | 团长为团队指定客服成员（`support_member_phone`） | `internal/store/team.go` + `internal/api/support.go` |
| 31 | 数据层 | SQLite 连接/迁移 | 启动自动建表迁移 + 首次 seed 种子商品/分类 | `internal/store/db.go` + `internal/store/catalog.go` |
| 32 | 数据层 | 商品/分类 | SQLite 持久化（两级类目 + 服务大类标记） | `internal/store/catalog.go` |
| 33 | 数据层 | 修改审计 | 商品修改写入 `product_edits`（字段/新旧值/操作人/时间） | `internal/store/db.go` |
| 34 | 数据层 | 购物车历史 | `cart_items` 表持久化 | `internal/store/cart.go` |
| 35 | 数据层 | 团队持久化 | `teams` / `team_members` / `team_create_requests` / `team_treasury_logs` 表 | `internal/store/team.go` |
| 35b | 数据层 | 地址/订单/提现/客服 | `addresses` / `orders` / `order_items` / `withdrawals` / `support_tickets` / `support_messages` 表 | `internal/store/order.go` + `internal/store/support.go` |
| 35c | 数据层 | 用户持久化 | `users` 表；启动 `LoadUsers` 恢复账号（订单/地址/提现可关联） | `internal/store/store.go` + `internal/store/db.go` |
| 36 | 数据层 | 内存存储 | 设备/消息/admin token 存内存（重启清空） | `internal/store/store.go` |
| 37 | 静态服务 | 图片静态服务 | `uploads/` 目录静态服务 `/uploads/*` | `internal/api/router.go` |
| 38 | 路由 | 路由注册 | Go 1.22 方法路由（如 `GET /path/{id}`） | `internal/api/router.go` |
| 39 | 调试 | 请求日志 | `DEBUG=1` 或 `-debug` 开启请求日志中间件 | `internal/api/debug.go` |


---

## 七、React 管理工作台功能

| 序号 | 功能模块 | 功能项 | 功能说明 | 代码位置 |
|----|---------|--------|---------|---------|
| 1 | 登录 | 管理员登录 | 密码登录（默认 admin123） | `src/pages/Login.tsx` |
| 2 | 路由守卫 | 登录守卫 | 未登录拦截跳转登录页 | `src/App.tsx` |
| 3 | 布局 | 侧边栏布局 | 侧边栏导航 + 路由 | `src/App.tsx` |
| 4 | 仪表盘 | 数据统计 | 核心业务数据统计 | `src/pages/Dashboard.tsx` |
| 5 | 仪表盘 | 数据库连接验证 | 校验数据库连接状态 | `src/pages/Dashboard.tsx` |
| 6 | 仪表盘 | 修改审计 | 查看商品修改审计记录 | `src/pages/Dashboard.tsx` |
| 7 | 用户管理 | 用户列表 | 查看管理平台注册用户 | `src/pages/Users.tsx` |
| 8 | 设备管理 | 设备列表 | 查看/管理设备 | `src/pages/Devices.tsx` |
| 9 | 设备管理 | 设备报警/移除 | 管理员触发报警、删除设备 | `src/pages/Devices.tsx` |
| 10 | 消息管理 | 消息列表 | 查看设备消息 | `src/pages/Messages.tsx` |
| 11 | 类目管理 | 增删改 | 分类新增/删除/编辑，持久化到服务器 | `src/pages/Categories.tsx` |
| 12 | 商品管理 | 编辑/新增/删除 | 商品增删改查 | `src/pages/Products.tsx` |
| 13 | 商品管理 | Excel 导入导出 | 浏览器端 SheetJS（xlsx）解析/生成 | `src/pages/Products.tsx` + `src/api.ts` |
| 14 | 商品管理 | 多图上传 | 商品多图上传（multipart → 服务器 uploads/） | `src/pages/Products.tsx` |
| 15 | 团队管理 | 团队列表 | 列表 / 成员 / 经营金额 / 解散 | `src/pages/Teams.tsx` |
| 16 | 账号管理 | admin 账号 | admin 创建/删除员工账号、角色/状态管理 | `src/pages/Accounts.tsx` |
| 16b | 订单管理 | 订单列表/发货 | 按状态筛选订单，发货填物流公司+单号（绑定物流） | `src/pages/Orders.tsx` |
| 16c | 提现审核 | 审核 | 提现列表（企业银行卡号），打款完成/驳回（自动退款） | `src/pages/Withdrawals.tsx` |
| 16d | 客服工作台 | 会话回复 | 后台客服收件箱：查看会话/回复/关闭 | `src/pages/Support.tsx` |
| 17 | 全局 | 请求封装 | 请求封装与商品 Excel 解析/生成 | `src/api.ts` |
| 18 | 全局 | 类型定义 | 数据类型定义（TS） | `src/types.ts` |
| 19 | 全局 | 全局样式 | 深蓝科技风样式 | `src/styles.css` |

---

## 八、API 一览

统一响应格式：`{ "code": 0, "msg": "ok", "data": ... }`（`code` 非 0 为错误）

### 8.1 公开接口（无需登录）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /health | 健康检查（含 db 连接状态） |
| GET | /api/v1/categories | 商品分类 |
| GET | /api/v1/products | 商品列表（?category= / ?keyword=） |
| GET | /api/v1/products/{id} | 商品详情 |

### 8.2 用户接口（Bearer Token）

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
| POST | /api/v1/team/support-member | 团长指定团队客服成员（接收/回复团队服务会话） |
| POST | /api/v1/team/treasury/withdraw | 团长提取团队金库到我的余额（仅团长） |
| POST | /api/v1/team/treasury/transfer | 团长从金库向团队成员余额转账（仅团长，目标须为成员） |
| GET | /api/v1/team/treasury/logs | 我的团队金库流水（income/withdraw/transfer） |
| GET/POST/PUT/DELETE | /api/v1/addresses(/{id}) | 收货地址（绑定账号，默认地址唯一） |
| POST | /api/v1/addresses/{id}/default | 设为默认地址 |
| POST | /api/v1/orders | 创建订单（服务器计价，存地址/商品快照） |
| GET | /api/v1/orders?status= | 我的订单列表 |
| GET | /api/v1/orders/{id} | 订单详情 |
| POST | /api/v1/orders/{id}/cancel | 取消订单（待付款） |
| POST | /api/v1/orders/{id}/pay | 支付订单（真实模式返回 wx.requestPayment 参数，微信回调 /api/v1/pay/notify 确认；模拟模式直接确认） |
| POST | /api/v1/pay/notify | 微信支付结果回调（验签 + AES-GCM 解密 → 订单置为 paid） |
| POST | /api/v1/orders/{id}/confirm | 确认收货 |
| POST | /api/v1/orders/{id}/refund | 无理由退款（退货期内，即时生效；关联待结算佣金自动取消） |
| GET/POST | /api/v1/withdrawals | 提现申请（真实模式收款账户=绑定 openid）/ 我的提现记录 |
| POST | /api/v1/user/bind-inviter | 被邀请人绑定邀请人（服务器持久化，幂等） |
| GET | /api/v1/user/invitees | 我邀请的好友（含消费/佣金统计） |
| GET | /api/v1/user/commissions | 我的佣金明细（pending 待结算/settled 已到账/cancelled 已取消） |
| POST | /api/v1/user/commission/demo | 单机演示：模拟好友下单佣金（延迟到账，与真实一致） |
| GET/POST | /api/v1/support/tickets(/{id}) | 客服会话（我的会话/创建，自动分配客服） |
| POST | /api/v1/support/tickets/{id}/messages | 用户发送客服消息 |
| GET | /api/v1/support/inbox | 团队客服收件箱（分配给我的会话） |
| POST | /api/v1/support/tickets/{id}/reply | 团队客服回复（校验分配归属） |
| POST | /api/v1/support/tickets/{id}/close | 团队客服关闭会话 |

### 8.3 管理接口（admin token，12h 过期）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/login | 管理员登录（admin123） |
| GET | /api/v1/admin/stats | 统计 |
| GET | /api/v1/admin/db-status | 数据库状态 + 审计 |
| GET | /api/v1/admin/users / devices / messages | 列表 |
| GET | /api/v1/admin/carts | 用户购物车历史 |
| GET | /api/v1/admin/teams / DELETE /{id} | 团队管理 |
| GET | /api/v1/admin/orders?status= | 全部订单（发货管理） |
| POST | /api/v1/admin/orders/{id}/ship | 后台发货（绑定物流公司与单号） |
| GET | /api/v1/admin/withdrawals | 全部提现申请（审核） |
| POST | /api/v1/admin/withdrawals/{id}/complete | 打款完成（真实模式经微信转账到零钱，模拟模式直接确认） |
| POST | /api/v1/admin/withdrawals/{id}/fail | 驳回提现（failed，自动退款到余额） |
| GET | /api/v1/admin/support/tickets?status= | 客服会话（后台客服收件箱） |
| GET | /api/v1/admin/support/tickets/{id} | 会话详情（含消息） |
| POST | /api/v1/admin/support/tickets/{id}/reply | 后台客服回复 |
| POST | /api/v1/admin/support/tickets/{id}/close | 后台关闭会话 |
| POST/DELETE | /api/v1/admin/devices/{id}/alarm / devices/{id} | 设备操作 |
| GET/POST/PUT/DELETE | /api/v1/admin/categories(/{id}) | 类目 CRUD |
| GET/POST | /api/v1/admin/products(/{id}) | 商品增删改查 |
| POST | /api/v1/admin/products/batch | 批量导入（JSON） |
| POST | /api/v1/admin/products/upload | 图片上传（multipart） |


---

## 九、数据库表（SQLite）

`server/data/app.db`（环境变量 `DB_PATH` 可改），启动自动迁移 + 首次 seed 种子数据：

| 表 | 说明 | 关键列 |
|----|------|--------|
| `categories` | 商品分类（两级 + 服务大类标记） | id, name, parent_id, sort, **is_service** |
| `products` | 商品（`category` 指向二级类目 ID） | id, name, price, original_price, emoji, colors, **images**, sales, category, tags, detail, **attributes**, **service**, **source_team** |
| `users` | 用户账号（持久化，重启后恢复；订单/地址/提现绑定 user_id） | id, phone, nick_name, avatar_url, role, balance, total_commission, promoter_code, **invited_by**（邀请人）, openid |
| `commissions` | 佣金结算（延迟到账） | id, user_id（获佣人）, order_id, amount, status(pending/settled/cancelled), paid_at, settle_at（=支付时间+退货期）, settled_at |
| `cart_items` | 用户购物车历史 | user_id, product_id, quantity |
| `teams` | 团队 | id, name, owner_phone, owner_name, business_amount, **treasury**（金库，90% 服务分成）, **support_member_phone**（团队指定客服成员） |
| `team_treasury_logs` | 团队金库流水 | id, team_id, type(income/withdraw/transfer), amount, target_phone, target_name, remark, created_at |
| `team_members` | 团队成员 | team_id, phone, nick_name, join_time |
| `team_create_requests` | 团员建团申请 | requester_phone, team_name, current_team_id, status(pending/approved/rejected) |
| `addresses` | 收货地址（绑定账号，默认地址唯一） | id, user_id, name, phone, region, detail, is_default |
| `orders` | 订单（支付 + 物流字段，地址快照） | id, order_no, user_id, status(pending/paid/shipped/done/canceled), total_amount, address_json, pay_method, pay_time, **transaction_id**, ship_company, ship_no |
| `order_items` | 订单商品快照（价格/属性/服务来源） | order_id, product_id, name, price, quantity, attrs, service, source_team |
| `withdrawals` | 提现申请（真实模式 account=用户 openid，模拟模式绑定企业银行卡号） | id, user_id, amount, fee, method, account, **bank_card_no**, status(processing/done/failed), apply_time |
| `support_tickets` | 客服会话（自动分配） | id, user_id, product_id, service, source_team, **assignee_type(admin/team)**, **assignee_phone**, status(open/closed), last_message |
| `support_messages` | 客服消息 | ticket_id, sender_type(user/admin/team), sender_name, content, read |
| `product_edits` | 商品修改审计 | product_id, field, old_value, new_value, operator, created_at |
| `sys_info` | 系统元信息 | key, value |

> 用户/地址/订单/提现/客服/团队/商品/分类/审计/购物车均存 SQLite（持久化，重启后账号可恢复关联）；设备/消息仍存服务器内存（重启清空）。

---

## 十、端间协同 / 关键数据流

| 数据流 | 链路 |
|--------|------|
| 商品数据流 | 工作台编辑/上传图片 → `POST /api/v1/admin/products/*` → SQLite `products` 表 → 小程序启动 `initProductCache` → `GET /api/v1/products` → 本地缓存（含图片完整 URL）→ 商城/详情/购物车读取展示 |
| 购物车历史 | 小程序加购/改数/移除 → `POST /api/v1/cart/sync`（记录商品 id 与数量）→ SQLite `cart_items`；小程序登录/冷启动/进入购物车页 → `GET /api/v1/cart` → 合并回本地（同商品数量取较大值）；工作台用户管理 → `GET /api/v1/admin/carts` 查看 |
| 用户资料 | 我的页编辑资料：`chooseAvatar` 选头像 → `POST /api/v1/user/avatar` → `uploads/avatar_*.jpg` → 保存 URL；昵称 `type=nickname` → `PUT /api/v1/user/profile` → 服务器 User 更新（本地 mall_user 同步） |
| 小程序登录认证 | 手机号+验证码(12345) → 本地登录 + 服务器登录取 token（`mall_server_token`）→ 请求带 `Authorization: Bearer` → 服务器校验（401 自动重登） |
| 工作台登录认证 | 管理员 admin123 → `POST /api/v1/admin/login` → admin token（12h 过期） |
| 订阅消息推送 | 手机 `wx.login` code → `/api/v1/auth/wx-login` → 服务器存 openid → 点「开启报警通知」`wx.requestSubscribeMessage` 授权 → `/notify/subscribe` 记额度 → 工作台触发 `alarm-all` → 服务器 `subscribeMessage.send` → 微信服务通知 |

---

## 十一、核心业务规则

| 规则项 | 内容 |
|--------|------|
| 会员体系 | 登录即会员，所有会员可邀请，无推广员层级 |
| 佣金比例 | 被邀请人订单实付金额 10%（`COMMISSION_RATE`，改 `miniProgram/utils/store.js`） |
| 佣金到账 | 延迟到无理由退货期满（`COMMISSION_SETTLE_DAYS`，默认 7 天，改 `server/.env`）；期内退款佣金取消 |
| 演示验证码 | 固定 12345（`SMS_CODE`） |
| 管理员密码 | 默认 admin123（服务器环境变量 `ADMIN_PASSWORD` 可改） |
| 建团资格 | 邀请人数 > 2 或所在团队经营金额 > 1w（服务器校验）；一用户仅属一个团队 |
| 团员建团 | 团员提交申请 → 现任团长审核通过 → 自动建新团并移出原团 |
| 服务商品 | 固定「服务」大类；仅后台与团队可发布，服务来源 = 团队名 / 官方服务 |
| 支付方式 | 服务器确认支付（弹窗模拟）；配置 `WECHAT_PAY_MCH_ID/MCH_KEY` 后对接真实微信支付 |
| 提现渠道 | 微信零钱 / 支付宝 / 银行卡；服务器事务内扣余额，绑定 `.env` 企业银行卡号，后台审核（done/failed，驳回自动退款） |
| 收货地址 | 绑定账号存服务器（SQLite），默认地址唯一；新增/设默认时自动取消其他默认，删除后自动补第一条为默认 |
| 客服分配 | 团队服务商品 → 团队指定客服成员（`teams.support_member_phone`，团长指定）接收/回复；普通商品/官方服务 → 后台客服（管理工作台） |
| 订单计价 | 以服务器商品表价格为准，不信任客户端金额；存地址/商品/属性快照（历史可追溯） |
| 存储分工 | 用户/地址/订单/提现/客服/团队/商品/分类/审计/购物车 → SQLite 持久化（重启后账号可恢复关联）；设备/消息/admin token → 服务器内存；小程序设备 → 本地 |

---

## 十二、配置项（服务器环境变量）

| 变量 | 默认 | 说明 |
|------|------|------|
| `ADDR` | `:8080` | 监听地址（监听 0.0.0.0 供真机访问） |
| `DB_PATH` | `data/app.db` | SQLite 路径 |
| `UPLOAD_DIR` | `uploads` | 商品图片目录 |
| `ADMIN_PASSWORD` | `admin123` | 管理密码 |
| `DEBUG` / `-debug` | - | 请求日志模式 |
| `WECHAT_APPID` | - | 微信 AppID（订阅消息必需） |
| `WECHAT_SECRET` | - | 微信 AppSecret |
| `WECHAT_TEMPLATE_ID` | - | 订阅消息模板 ID |
| `WECHAT_PAY_MCH_ID` | - | 微信支付商户号 |
| `WECHAT_PAY_MCH_KEY` | - | 微信支付 API 密钥 |
| `WECHAT_PAY_BANK_CARD` | - | 企业银行卡号（提现打款收款卡，服务器写入提现记录） |
| `WECHAT_PAY_NOTIFY_URL` | - | 支付结果回调地址 |

> `main.go` 启动时自动加载 `server/.env`（环境变量优先）。

---

## 十三、已知限制与演进方向

| 项 | 现状 | 演进建议 |
|----|------|---------|
| 设备/消息 | 服务器内存存储，重启清空 | 迁移到 SQLite/MySQL |
| 用户 | 已持久化 SQLite（`users` 表），重启后可恢复账号与订单/地址/提现关联 | 接入正式账号体系后可对接短信验证 |
| 小程序设备 | 存小程序本地，与服务器设备未打通 | 统一走 `/api/v1/devices` |
| 小程序登录 | 本地验证码 12345 | 接服务器 `/auth/login` + 真实短信 |
| 支付 | 服务器确认支付（模拟）；已预留 `WECHAT_PAY_MCH_ID/MCH_KEY` 配置 | 配置商户证书后接 `wx.requestPayment` 真实下单 |
| 提现打款 | 后台审核 + 绑定企业银行卡号（模拟打款记录） | 配置商户后走真实企业付款到银行卡 API |
| 订阅消息 | 需企业主体 + 模板配置 | 配置 `WECHAT_*` 后生效 |
| 商品图片 | 本地 `uploads/` | 生产用对象存储（COS/OSS） |
| Excel 处理 | 浏览器端 SheetJS | 数据量巨大时改服务器端 |
| 工作台认证 | admin/staff 角色 | 可扩展更多角色/权限 |

