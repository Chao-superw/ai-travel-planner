# 前端接入说明

本地 API 默认地址是 `http://127.0.0.1:8080`。启动 travel RPC 后运行 API 进程；前端开发源默认允许 `http://localhost:5173` 与 `http://127.0.0.1:5173`。`GET /healthz` 只表示 API 进程存活，`GET /readyz` 成功才表示 travel RPC 可用。完整机器可读契约位于 `GET /openapi.yaml` 和 [openapi.yaml](./openapi.yaml)。

所有 JSON 字段使用 `snake_case`。成功响应统一为：

```json
{"data": {}, "request_id": "request-id"}
```

同步 HTTP 错误响应为 `{"code":"...","message":"...","request_id":"request-id","field_errors":{}}`，其中 `field_errors` 可省略，认证限流另带数字 `retry_after`（秒），顶层响应不含 `status` 字段。400 表示请求格式或字段无效，401 表示登录凭据缺失或失效，403 表示需要验证邮箱或权限不足，404 表示资源不存在或不可见，409 表示邮箱/版本/幂等冲突，422 表示业务约束无法满足，429 表示认证限流或任务配额限制，503 表示同步依赖或 RPC 暂不可用，504 表示同步调用超时。网关会把传输错误脱敏，不向前端暴露供应商 URL。排障时保留 `request_id`。请求正文最大 1 MiB，未知字段、同一 JSON 后追加内容都会返回 400。

## 邮箱认证与旧账号迁移

本节描述前端接入契约。2026-09-07 经用户追加授权，`FE/` 已完成邮箱登录、验证码注册和已有未验证会话的绑定页面；按用户要求，登录页不展示用户名登录或迁移入口。后端的 legacy 接口仍作为兼容契约保留。原来的 `POST /auth/register`、`POST /auth/login` 请求体已经改变：向这两个接口传 `username` 会返回 400。下表路径均以 `/api/v1` 开头；`username` 在用户响应中仅保留为兼容展示名，新用户展示名由服务端生成。

| 方法与路径 | 请求正文 | 成功时的 data |
|---|---|---|
| `POST /auth/register/code` | `{email}` | 200 `{challenge_id, expires_in, retry_after}` |
| `POST /auth/register` | `{email, password, code, challenge_id}` | 201 User，未登录 |
| `POST /auth/login` | `{email, password}` | 200 `{token, expires_at, user}` |
| `POST /auth/legacy/login` | `{username, password}` | 200 Session，仅未绑定邮箱的旧账号可用 |
| `POST /me/email/code` + Bearer | `{email}` | 200 Challenge |
| `POST /me/email` + Bearer | `{email, code, challenge_id}` | 200 绑定后的 User |
| `GET /me` + Bearer | 无 | 200 User |
| `POST /auth/logout` + Bearer | 无 | 200 `{logged_out:true}` |

User 包含 `id, username, role, email, email_verified`。旧账号未绑定时为 `email=""`、`email_verified=false`；绑定前只允许当前用户查询、注销、发绑定码和绑定邮箱，地点、任务、行程以及管理员业务接口均返回 403 `EMAIL_VERIFICATION_REQUIRED`。前端启动和恢复会话后先查询 `/me`，发现未验证就进入绑定页，保留当前 token。绑定成功保留原用户 ID、密码、角色、行程和现有会话，再查询 `/me` 更新界面。绑定后旧用户名登录失效，以后使用邮箱和原密码登录；当前版本不提供已绑定邮箱换绑或密码重置接口。

邮箱只接受 ASCII 单一地址，如 `Traveler+trip@example.com`，拒绝显示名、多地址与控制字符。后端去首尾空白、只把域名转为小写，保留 `@` 前大小写、点号和别名，不合并 QQ 别名或 Gmail 点号。规范化后总长最多 254 字节、本地部分最多 64 字节。前端不要把整个邮箱转为小写。密码要求 **8–72 UTF-8 字节**，不会自动截断，不能按 JavaScript `string.length` 判断；示例使用 `TextEncoder`。验证码必须以字符串传入，保留开头的零。

发码成功返回 challenge ID、剩余有效秒数和重发等待秒数，API 从不返回验证码。默认有效期 600 秒、重发间隔 60 秒、同一有效 challenge 最多 5 次错误猜测；始终以返回值驱动倒计时。同邮箱成功重发后，新码取代旧码；用户更改邮箱时应清空旧的 challenge 与验证码输入。已注册邮箱也走相同的发邮件流程，收到 200 不能据此判断账号是否存在。只有持有有效验证码并提交注册时，才可能收到 `EMAIL_ALREADY_REGISTERED`；该证明会被消费，已有密码不会被覆盖。

下面的 TypeScript 展示新注册与旧账号绑定的调用顺序。`proofEmail` 应保留发码时输入的邮箱，密码与验证码只保存在当前表单内。发码由按钮事件单次触发，提交注册由用户读到邮件后触发。

```ts
type User = { id: string; username: string; role: "user" | "admin";
  email: string; email_verified: boolean };
type Challenge = { challenge_id: string; expires_in: number; retry_after: number };
type Session = { token: string; expires_at: string; user: User };
type APIErrorBody = { code: string; message: string; request_id: string;
  retry_after?: number; field_errors?: Record<string, string> };
class APIError extends Error {
  constructor(readonly status: number, readonly body: APIErrorBody) {
    super(body.message);
  }
}
const base = "http://127.0.0.1:8080/api/v1";
async function api<T>(path: string, method: "GET" | "POST", body?: object,
  token?: string): Promise<T> {
  const response = await fetch(base + path, {
    method, cache: "no-store",
    headers: { ...(body ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const payload = await response.json();
  if (!response.ok) throw new APIError(response.status, payload);
  return payload.data as T;
}
const requestRegistrationCode = (email: string) =>
  api<Challenge>("/auth/register/code", "POST", { email });
async function finishRegistration(proofEmail: string, password: string,
  code: string, challenge: Challenge): Promise<Session> {
  const bytes = new TextEncoder().encode(password).length;
  if (bytes < 8 || bytes > 72) throw new Error("密码须为 8–72 UTF-8 字节");
  await api<User>("/auth/register", "POST", {
    email: proofEmail, password, code, challenge_id: challenge.challenge_id,
  }); // 201 返回 User，不含 token
  return api<Session>("/auth/login", "POST", { email: proofEmail, password });
}
const legacyLogin = (username: string, password: string) =>
  api<Session>("/auth/legacy/login", "POST", { username, password });
const requestBindingCode = (email: string, token: string) =>
  api<Challenge>("/me/email/code", "POST", { email }, token);
const finishBinding = (proofEmail: string, code: string,
  challenge: Challenge, token: string) =>
  api<User>("/me/email", "POST", {
    email: proofEmail, code, challenge_id: challenge.challenge_id,
  }, token);
const currentUser = (token: string) => api<User>("/me", "GET", undefined, token);
```

普通邮箱登录直接提交 `/auth/login`，不需要每次重新发码。登录成功保存 `data.token`，服务端会话有效 8 小时，之后每个受保护请求都发送 `Authorization: Bearer <token>`。不要把 token、密码、邮箱验证码写入 URL、日志或持久化分析事件，也不要缓存认证请求及响应；所有 `/api/v1/auth/*` 和 `/api/v1/me`、`/api/v1/me/*` 响应（包括错误）均带 `Cache-Control: no-store`。注销成功为 `data.logged_out=true`，前端清除本地会话。权限与资源归属由后端验证。

| HTTP / code | 前端处理 |
|---|---|
| 400 `INVALID_INPUT` | 展示字段/格式提示；不要继续原样提交 |
| 400 `INVALID_VERIFICATION_CODE` | 统一表示错误、过期、已使用、用途/邮箱/账号不匹配或次数耗尽；允许检查输入或重新获取 |
| 401 `INVALID_CREDENTIALS` | 邮箱/旧用户名或密码错误，统一提示，不判断账号是否存在 |
| 401 `UNAUTHORIZED` / `UNAUTHENTICATED` | 缺少、失效或已撤销的 Bearer，清理登录态并要求登录 |
| 403 `EMAIL_VERIFICATION_REQUIRED` | 保留 token，进入旧账号邮箱绑定流程 |
| 409 `EMAIL_ALREADY_REGISTERED` | 引导使用已有邮箱账号登录；不会重置密码 |
| 409 `EMAIL_ALREADY_BOUND` | 当前账号已绑定，刷新 `/me`；不要继续提交换绑 |
| 429 `AUTH_RATE_LIMITED` | 按数字 `body.retry_after` 暂停相应操作；响应头 `Retry-After` 同为秒数，CORS 已允许读取 |
| 503 `MAIL_UNAVAILABLE` | 发码失败，停止自动重试；保留先前仍有效的 challenge，等待冷却后由用户重发 |
| 503 `AUTH_UNAVAILABLE` / `DATABASE_UNAVAILABLE` / `DEPENDENCY_UNAVAILABLE` / `RPC_UNAVAILABLE` | 服务暂不可用，不清理现有登录态；稍后由用户重试 |
| 504 `TIMEOUT` | 请求结果未确认，先查询可确认的状态，避免循环提交 |

邮箱发送最大等待预算为 SMTP 10 秒、RPC 14 秒、网关 15 秒；前端请求超时应留出网络余量。后端不自动重发邮件。SMTP 超时也可能发生在邮件实际送达之后，此时该次验证码可能不可用；不要把收到邮件当作这次发码请求成功。发码失败不使原先仍有效的验证码失效，但仍计入冷却及限流。

默认发码限额为邮箱/IP/全局每小时 5/100/500 次，验证尝试为每小时 30/300/3000 次，登录为账号/IP/全局每 15 分钟 10/100/1000 次；注册与绑定共享发码/验证限额，正常与旧用户名登录共享登录限额。限流取多个维度中的实际等待时间，前端以 `retry_after` 为准。IP 来自网关 TCP 连接，当前不信任 `X-Forwarded-For` 等转发头；部署在代理后，IP 额度会按代理连接地址聚合。

## 行程任务与编辑

创建规划任务、任务重试、手动修改和局部重规划必须携带 1–128 字节的 `Idempotency-Key`。建议每次用户操作生成一个新键，网络重试复用同一个键。任务创建返回 202，`data.status_url` 可直接轮询。建议每 2 秒查询一次，页面离开或达到产品超时后停止；最终状态是 `succeeded`、`failed`、`conflicted` 或 `interrupted`，中间状态是 `queued`、`running`。异步任务已经返回 202 后，执行错误不会变成后续 HTTP 4xx/5xx：查询任务仍返回 HTTP 200，失败分类位于 `data.error.code/message/status`。其中 `conflicted` 通常要求重新读取当前版本后重新提交，`failed` 或 `interrupted` 可按后端规则显式重试。成功后使用 `data.trip_id` 请求 `GET /api/v1/trips/{id}`；不要把任务响应当成完整行程。

新建规划的推荐请求如下。`budget_cents` 是输入预算，`budget_scope=per_person` 表示每人预算；`budget_total_cents` 由服务端计算，客户端不得提交。

```json
{
  "constraints": {
    "city": "杭州市",
    "start_date": "2026-10-01",
    "end_date": "2026-10-03",
    "party_size": 2,
    "budget_cents": 200000,
    "budget_scope": "per_person",
    "interests": ["文化", "美食"],
    "pace": "balanced",
    "transport": "transit"
  }
}
```

`transport=walking` 只查询步行；`transit` 表示公交优先。只有公交明确无方案时，后端才尝试一次真实步行查询，并且仅接受不超过 1500 米且不超过 1500 秒的短程路线；结果会显示实际 `route.mode=walking` 并带告警。地图鉴权、额度、超时等故障不会触发此回退。前端请以每段路线的实际 mode 渲染。

创建接口只持久化任务并快速返回 202，不在 HTTP 请求内等待规划。任务排队最多 300 秒，领取后执行最多 300 秒；两段预算彼此独立，因此从提交到终态最坏可接近 600 秒。模型单次非流式 HTTP 请求最多 120 秒，任务最多调用初次生成和一次修复；剩余执行预算还需容纳校验、算路和持久化。等待时展示真实 `stage`，不要在 60 秒或 300 秒时自行判定整个任务失败。

金额单位始终为“分”。`known_total_cents` 只累计已知费用；未知价格以省略或 `null` 表示，并计入 `unknown_count`。首版还会把没有明确起终点的餐饮/住宿交通、每日首末段城市接驳作为未知交通项纳入 `by_day` 和 `by_category`。只要存在这些未知项，`complete=false` 且 `within_budget` 省略，前端不能展示“未超预算”。`budget_scope=per_person` 的人数换算只由服务端执行一次。

行程列表和版本列表只返回摘要，不含 `activities`、`routes`。完整行程查询返回约束、活动地点快照和路线。地点 `location` 为“经度,纬度”，坐标系为 GCJ-02。路线是后端在 `queried_at` 对已知活动位置和出发参数的查询结果，不保证未来出行时刻的实时路况。

手动修改 `PATCH /api/v1/trips/{id}` 只提交 `expected_version` 及完整的可编辑 `plan`。每个活动只允许 `id,date,start_minute,end_minute,kind,place_id,title,reason`；不要回传地点快照、费用、路线或告警。服务端保留未变化活动的可信资料，并重新计算变化部分的地点快照、费用和路线；该任务 `kind=manual_edit`，不调用模型。

局部重规划的 `scope.editable_item_ids` 和 `locked_item_ids` 必须取自刚查询到的实际活动 ID，例如用户选中响应中的 `activity-id-2`，就提交该 ID，不能由前端编造。示例：

```json
{
  "expected_version": 3,
  "scope": {
    "date": "2026-10-02",
    "start_minute": 540,
    "end_minute": 1080,
    "editable_item_ids": ["activity-id-2"],
    "locked_item_ids": ["activity-id-3"]
  },
  "instruction": "下午减少步行，保留已经锁定的晚餐。"
}
```

写入时若返回 409 `VERSION_CONFLICT`，重新获取当前行程并让用户基于新版本确认修改，随后使用新的 `expected_version` 和新的幂等键提交。不要盲目覆盖。

如果异步重规划返回 `data.error.code=SCOPE_CONFLICT`，表示新方案影响了范围外交通费用或衔接。展示 `message`，让用户重新选择更宽的修改范围后提交新任务；不要自动扩大 scope 或用同样参数连续重试。真实验证已通过“第二天整天修改、其他日期保持不变”；下午窄范围案例触发过这一保护，详见 [验证记录](verification.md)。

管理员创建地点资料时，`id` 必须来自此前的地点搜索，服务端会核实供应商身份。更新地点时路径 ID 为准，可编辑字段采用完整替换语义；未知营业时间或费用传 `null`，不要发送 provider、provider_poi_id、name、location 等只读身份字段。
