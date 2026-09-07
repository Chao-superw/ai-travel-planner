# AI 旅行行程规划师：前端设计与实现约定

日期：2026-09-07
状态：用户已确认技术栈、页面范围、联调方式与工程位置。按本文默认值进入实施；实现完成情况单独记录，不把设计视为已验证结果。

本文保留设计过程。字段与接口以 `BE/docs/openapi.yaml`、`BE/docs/frontend-integration.md` 为准，后端行为约束以 `BE/docs/verification.md` 为准。前端只访问 HTTP 网关，不直接调用高德 Web 服务或模型。

## 1. 已确认的目标与决策

- 交付目标：从零搭建真实 React 前端工程，对接现有 Go 后端（`http://127.0.0.1:8080`），实现 MVP 全部页面内容。
- 产品定位：纯「AI 行程生成工具」，不做种草社区。视觉沿用已确认的 demo 风格（Airbnb 克隆骨架 + Instagram 极简）。
- 工程位置：`小学期/FE/`，与 `BE/` 平级。
- 框架：Vite + React（SPA），react-router 多路由。
- 技术栈：Tailwind CSS + shadcn/ui（组件）+ @tanstack/react-query（数据请求、轮询、缓存）+ react-hook-form + zod（向导表单校验）。
- 联调方式：直连真实后端。后端已本地起且 GLM/高德 Key 已配齐，生成链路可真实产出行程。
- 地图：高德 JS API v2，真实渲染活动点位与每段路线折线。先复用后端那把高德 Key；若浏览器控制台报 `INVALID_USER_KEY`，改用「Web端(JS API)」类型 Key，代码零改动（Key 由单个环境变量注入）。
- 登录态：`data.token` 存 localStorage，持久登录；受保护请求带 `Authorization: Bearer <token>`。token 不写入 URL、日志、分析事件。

## 2. MVP 范围（YAGNI）

**纳入本期：**

1. 首页：Hero + 热门线路模板（一键生成）+ 快速偏好入口。
2. 需求向导 Modal：收集结构化约束并做 zod 校验。
3. 生成中页：创建任务→轮询任务状态→跳转结果。
4. 行程详情：时间线（活动）+ 高德地图（点位 + 路线折线）+ 预算面板。
5. 注册 / 登录 / 退出。
6. 我的行程：摘要分页列表，点进取完整详情。

**暂不纳入（后端有接口，列为二期）：** 局部重规划（replan）、手动修改（manual_edit）、历史版本对比、管理员地点资料管理。此边界由用户确认，以控制 MVP 体量。

## 3. 页面与路由

| 路由 | 页面 | 鉴权 | 主要接口 |
| --- | --- | --- | --- |
| `/` | 首页（Hero + 模板 + 偏好入口） | 公开 | 无（模板为前端内置示意，点击后走登录门槛） |
| `/login`、`/register` | 登录 / 注册 | 公开 | `POST /api/v1/auth/login`、`/register` |
| `/planning/:jobId` | 生成中（轮询） | 需登录 | `POST /api/v1/planning-jobs`、`GET /api/v1/planning-jobs/{id}` |
| `/trips/:id` | 行程详情 | 需登录 | `GET /api/v1/trips/{id}`、`GET /api/v1/trips/{id}/budget` |
| `/trips` | 我的行程列表 | 需登录 | `GET /api/v1/trips` |

- 向导以 Modal 承载，可在首页或列表页任意处唤起；提交成功（202）后跳到 `/planning/:jobId`。
- 未登录访问受保护路由重定向到 `/login`，登录后回跳原目标。

## 4. 目录结构

```text
FE/
  index.html
  vite.config.ts            开发代理与端口 5173
  tailwind.config.ts
  .env.example              VITE_API_BASE_URL、VITE_AMAP_KEY（占位，不含真实值）
  src/
    main.tsx                挂载 + QueryClientProvider + Router
    App.tsx                 路由表 + 受保护路由
    lib/
      http.ts               fetch 封装：base URL、Bearer、统一响应/错误解析
      idempotency.ts        生成/复用 Idempotency-Key
      money.ts              分↔元、预算文案（complete=false 不显示“未超预算”）
      time.ts               start_minute/end_minute → 时刻文案
      amap.ts               高德 JS SDK 懒加载与地图实例封装
    api/                    按资源分文件的 react-query hooks
      auth.ts  jobs.ts  trips.ts  budget.ts
    features/
      home/                 Hero、模板卡片、快速偏好入口
      wizard/               向导 Modal + zod schema
      planning/             生成中：轮询、stage 展示、错误分类
      trip/                 时间线、地图、预算面板
      trips/                我的行程列表
      auth/                 登录/注册表单
    components/ui/          shadcn/ui 生成的基础组件
    store/                  轻量 auth 状态（token/user）
```

单元边界：`lib/http.ts` 是唯一出网入口，`api/*` 只组织查询/变更并归一化数据，`features/*` 只消费 hooks 不直接 fetch。这样接口变更集中在 `lib` + `api`，页面不受影响。

## 5. 数据流与后端契约映射

### 5.1 统一约定
- 成功响应统一取 `body.data`；始终保留 `request_id` 便于排障。
- 同步错误按状态码分类：400 参数/字段（读 `field_errors` 回填表单）、401 登录失效（清 token 跳登录）、403 权限、404 不存在、409 冲突、422 业务约束、429 配额、503 依赖不可用、504 超时。
- JSON 字段一律 snake_case；请求体 < 1 MiB，不发未知字段。

### 5.2 鉴权
- 登录/注册成功保存 `data.token` 与 `data.user` 到 localStorage 与内存 store。
- 受保护请求统一附 `Authorization: Bearer <token>`；收到 401 即登出并回跳。
- 退出调用 `POST /api/v1/auth/logout`，成功 `data.logged_out=true` 后清本地态。

### 5.3 创建生成任务（异步核心）
- `POST /api/v1/planning-jobs`，请求体为 `{constraints: ConstraintsInput}`，必带 1–128 字节 `Idempotency-Key`；每次用户提交生成新键，网络重试复用同键。
- `budget_scope=per_person` 时提交 `budget_cents` 为每人预算；`budget_total_cents` 由服务端计算，前端不提交。
- 返回 202，取 `data.status_url`（等价 `GET /api/v1/planning-jobs/{data.id}`）。

### 5.4 轮询
- react-query `refetchInterval` 每 2 秒查一次任务，命中终态即停。
- 中间态 `queued`/`running`：展示真实 `data.stage` 文案，不在 60s/300s 自行判失败；产品侧设一个较宽的最长等待（接近 600s）后给出「仍在处理，可稍后在我的行程查看」的出口。
- 终态处理：
  - `succeeded`：用 `data.trip_id` 跳 `/trips/{trip_id}`。
  - `failed`/`interrupted`：展示 `data.error.message`，提供「重试」按钮，走 `POST /api/v1/planning-jobs/{id}/retries`（新幂等键）。
  - `conflicted`：提示需重新读取当前版本再提交（MVP 生成场景少见，仍给明确文案）。
- 异步失败不表现为 HTTP 4xx/5xx：轮询本身返回 200，失败信息在 `data.error.code/message/status`。

### 5.5 行程详情
- `GET /api/v1/trips/{id}` 返回完整 `Trip`：`constraints` + `plan{title,summary,activities,routes,warnings}`。
- 时间线：按 `activity.date` + `start_minute/end_minute` 排布，展示 `title`、`reason`、`place`、`costs`。
- 地图：`place.location` 是 GCJ-02「经度,纬度」，高德 JS 默认即 GCJ-02，直接下点；`route.polyline` 画折线，按每段 **实际 `route.mode`** 渲染（transit/walking 用不同样式），展示 `distance_m`/`duration_s`/`summary`；渲染以每段真实 mode 为准，不用请求偏好。
- 预算：`GET /api/v1/trips/{id}/budget`，分→元展示 `budget_total_cents`/`known_total_cents`/`unknown_count` 与 `by_day`/`by_category`。`complete=false` 时 `within_budget` 省略，**不显示「未超预算」**，改为「含 N 项待确认费用」。同时展示 `plan.warnings`。

### 5.6 我的行程列表
- `GET /api/v1/trips?page=&page_size=`，只返回摘要（无 activities/routes），展示 `title`/`summary`/`constraints`/`created_at`；点进详情再取完整数据。

## 6. 错误与边界处理
- 统一错误 toast/inline：优先展示后端 `message`，附 `request_id`（便于用户反馈）。
- 表单 400：把 `field_errors` 映射回 react-hook-form 字段错误。
- 401：静默登出 + 跳登录 + 回跳。
- 429：提示配额限制，禁用重复提交。
- 503/504：提示依赖暂不可用/超时，可稍后重试；网关已脱敏，不暴露供应商信息。
- 地图 Key 失效（`INVALID_USER_KEY`）：地图区降级为「地图不可用」占位 + 文字路线信息，不阻塞行程展示。

## 7. 视觉与交互基调
- 布局沿用已确认 demo：fixed 顶栏、药丸搜索/入口、横滑快速偏好条、响应式卡片网格、Modal 向导、生成中骨架屏、详情时间线 + 地图联动。
- 配色与字体走 Instagram 极简：纯白背景、系统无衬线、1px 细分割线、极简品牌色点缀。
- 像素代码风作为生成中的加载动效/品牌彩蛋，不作为主框架。

## 8. 环境与运行
- 开发端口固定 5173（后端 CORS 白名单默认已放行 `localhost:5173`、`127.0.0.1:5173`）。
- `.env.example` 提供 `VITE_API_BASE_URL=http://127.0.0.1:8080`、`VITE_AMAP_KEY=`（占位，真实值本地填 `.env.local`，不入库）。
- 不提交 `.env.local`、真实 Key、token 或任何凭据。

## 9. 不做的事（本期显式排除）
- 不做 SSR/Next（已定 SPA）。
- 不做 replan/manual_edit/版本历史/管理员地点管理页面。
- 不做离线缓存、多人协作、通用网页搜索、天气与在线预订。
- 未经用户明确要求不提交或推送代码，不删除已有文件。
