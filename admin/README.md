# 安全监护 · 管理工作台

基于 **React 18 + TypeScript + Vite** 的管理后台，用于管理 Go 服务器（`../server`）的数据。

## 功能

- 🔐 管理员登录（默认密码 `admin123`，服务器环境变量 `ADMIN_PASSWORD` 可改）
- 📊 仪表盘：用户/设备/报警/消息/商品统计 + **数据库连接验证** + 商品修改审计
- 👥 用户管理：手机号、角色、余额、佣金、openid
- 📟 设备管理：设备列表、触发报警、移除设备
- 💬 消息管理：报警/提醒/通知消息列表、删除
- 🏷️ 类目管理：**类目不写死**，支持新增/编辑/删除（持久化到 SQLite）
- 🛒 商品管理：价格/原价/销量编辑 + **Excel 批量导入/导出** + 新增/删除商品

## 快速启动

```bash
# 1. 先启动 Go 服务器（见 ../server/README.md）
cd ../server && go run main.go

# 2. 启动工作台（开发模式，端口 5173，/api 代理到 8080）
cd ../admin
npm install
npm run dev
```

打开 http://localhost:5173 即可。

## 构建与部署

```bash
npm run build        # 产物在 dist/
npm run preview      # 本地预览构建产物
```

生产部署时通过环境变量指定 API 地址：

```bash
VITE_API_BASE=https://your-server.com npm run build
```

## 目录结构

```
admin/
├── src/
│   ├── api.ts          # 请求封装（Bearer token 自动附加）
│   ├── types.ts        # 数据类型定义
│   ├── App.tsx         # 布局 + 路由
│   ├── pages/          # 登录/仪表盘/用户/设备/消息/商品
│   └── styles.css      # 全局样式
├── vite.config.ts      # /api 代理到 Go 服务器
└── package.json
```

## 管理接口（服务器侧）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/login | 管理员登录，返回 admin token（**12 小时有效**） |
| GET | /api/v1/admin/stats | 数据统计 |
| GET | /api/v1/admin/db-status | **数据库连接状态** + 最近商品修改审计 |
| GET | /api/v1/admin/users | 用户列表 |
| GET | /api/v1/admin/devices | 设备列表 |
| POST | /api/v1/admin/devices/{id}/alarm | 触发设备报警 |
| DELETE | /api/v1/admin/devices/{id} | 移除设备 |
| GET | /api/v1/admin/messages | 消息列表 |
| DELETE | /api/v1/admin/messages/{id} | 删除消息 |
| GET | /api/v1/admin/categories | 类目列表 |
| POST | /api/v1/admin/categories | 新增类目 |
| PUT | /api/v1/admin/categories/{id} | 更新类目 |
| DELETE | /api/v1/admin/categories/{id} | 删除类目 |
| GET | /api/v1/admin/products | 商品列表 |
| POST | /api/v1/admin/products | 新增商品 |
| POST | /api/v1/admin/products/batch | **批量导入**（前端解析 Excel 后提交 JSON） |
| PUT | /api/v1/admin/products/{id} | 更新商品（**强校验 + 审计落库**） |
| DELETE | /api/v1/admin/products/{id} | 删除商品 |

## Excel 导入导出（前端处理）

Excel 的解析/生成在**浏览器端完成**（SheetJS），服务器只接收结构化 JSON：

- **导出**：前端拉取商品 JSON → SheetJS 生成 xlsx → 浏览器直接下载（不经过服务器）
- **导入**：选择文件 → 前端 SheetJS 解析 → 提交 `POST /api/v1/admin/products/batch` → 服务器**强校验 + 审计落库**
- 列结构：`id, 名称, 描述, 价格, 原价, 图标, 颜色(JSON), 销量, 类目ID, 标签(JSON), 详情(JSON)`
- `id` 已存在则**更新**，不存在则**新增**；导入结果提示新增/更新/失败数量

> 架构原因：数据量小的后台网页场景，浏览器端解析利用用户设备算力、只传小体积 JSON，服务器零 Excel 负担；服务器仍保留最终校验（不可信任前端数据）。

## 数据库

服务器内置 **SQLite**（`server/data/app.db`）持久化**商品修改审计**：

- 商品修改接口对价格/原价/销量做**强校验**（价格 0~999999、原价不低于现价、销量非负），非法请求被拒绝
- 每次修改写入 `product_edits` 表（字段、旧值→新值、操作人、时间），**服务器重启不丢失**
- 仪表盘「数据库连接」卡片展示连接状态、驱动、路径、数据表、审计记录与最近修改
