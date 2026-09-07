# AI 旅行行程规划师 · 后端

提供 **Hertz HTTP 网关 + Kitex Travel RPC + Kitex Planner RPC** 三个应用进程，使用 PostgreSQL 持久化、高德查询景点及路线、GLM-5.3-Flash 生成和局部重排行程。本目录包含后端和接入文档，配套前端位于 `../FE`。

所有后端文件位于 `BE/`。下文的构建、启动和验证命令均在该目录执行；从项目根目录进入：

```sh
cd BE
```

## 前端先看

- [前端接入指南](docs/frontend-integration.md)：登录、创建任务、轮询、局部修改、错误处理和预算图表。
- [OpenAPI 3.0 契约](docs/openapi.yaml)：可导入 Apifox/Postman；服务启动后也可 GET `/openapi.yaml`。
- [HTTP 示例](docs/examples.http)：邮箱注册、旧账号绑定与业务接口的调用方式。
- [验证记录](docs/verification.md)：实际通过的验证与尚未验证的环境。

本地接口地址 `http://127.0.0.1:8080`。前端通过 HTTP 网关访问业务接口，不直接调用 Kitex 或模型；景点和交通路线由后端查询，浏览器地图展示加载高德 JS API。默认允许 `http://localhost:5173` 和 `http://127.0.0.1:5173`；其他开发端口加入 `CORS_ORIGINS`。

## 功能

- 邮箱验证码注册、邮箱密码登录/退出、旧用户名账号绑定迁移、8 小时服务端会话、普通用户与管理员权限、私人行程隔离。
- 单城市、1～7 天、1～8 人，支持人均/全团人民币预算。
- 高德景点与博物馆候选、步行/公交优先路线、交通时间与日程校验。
- 异步生成、范围受限的自然语言修改、无需模型的手动修改。
- 不可变历史版本、版本冲突检测、持久化幂等、任务租约/过期恢复。
- 费用按日/类别统计。未知价格及未定位市内接驳单列未知项，不伪装成零费用。

## 启动方式一：Docker Compose

需要 Docker Compose。按 `.env.example` 补齐配置：`POSTGRES_PASSWORD`、`BIGMODEL_API_KEY`、`AMAP_API_KEY`、随机长字符串 `PLANNER_SERVICE_TOKEN`，以及下文 SMTP 与 `AUTH_CODE_HMAC_KEY`。所有 `REPLACE_WITH_*` 都是明确占位符，启动前必须替换；原生运行时 `DATABASE_URL` 中的密码须与实际数据库一致。**已有 `.env` 时保留并补齐，勿用示例覆盖**；其中可能只有 SMTP/HMAC 配置，数据库、模型和服务间凭据仍需从原有运行环境加载。仅在首次配置且文件不存在时可复制模板：

```sh
[ -e .env ] || cp .env.example .env
# 补齐 .env 或当前 shell 的运行变量后：
docker compose up --build -d
curl http://127.0.0.1:8080/readyz
```

仅 HTTP 8080 暴露到主机回环地址；数据库和 RPC 仅容器内部互通。数据库使用命名持久卷；`docker compose down` 保留数据卷。修改数据库口令时须同时处理已有数据库账户，而不是仅修改配置文件。

普通用户先申请邮箱验证码，再注册和登录。管理员命令仅提升**已经验证邮箱的现有账号**，不会创建账号或设置密码；先完成普通注册/旧账号绑定，再执行：

```sh
docker compose exec -e ADMIN_EMAIL='verified-admin@example.com' travel /app/bin/admin
```

## 启动方式二：本机 Go

推荐 Go 1.26.4（本机验证版本）；模块要求 Go 1.25+。需要 PostgreSQL 16。已安装数据库时自行设置 `DATABASE_URL`；没有时可使用开发工具启动真实的临时 PostgreSQL：

```sh
make build
make dev-db
```

开发数据库监听 `127.0.0.1:15432`，数据位于 `.local/postgres`。`make dev-db` 使用公开的本地演示口令 `local-travel-only`；连接该工具创建的数据库时，将 `DATABASE_URL` 的密码占位符替换为此口令，不能仅改模板就改变已有数据库密码。首次启动会从 Maven Central 下载数据库二进制，需联网；这是本地开发工具，不是生产数据库管理器。保持该终端运行。

其他终端先加载原有的 `DATABASE_URL`、服务间凭据、模型/地图配置，再补充 `.env` 中的 SMTP/HMAC 配置（其内容必须是你信任的本地配置）。原生 Go 程序不自动读取 dotenv 文件；只有 SMTP 设置的 `.env` 不能替代完整运行环境：

```sh
set -a
. ./.env
set +a
```

分别运行：

```sh
./bin/planner
./bin/travel
./bin/api
```

`travel` 启动时自动执行有版本记录的增量迁移：为 `users` 增加邮箱列/唯一索引，并新增认证挑战和限流表；保留已有账号、密码、角色、会话和行程，不清库。迁移记录在 `schema_migrations`，重启跳过已经应用的 DDL。`planner` 和 `travel` 必须使用相同的 `PLANNER_SERVICE_TOKEN`。缺少模型/地图 Key 时仍可使用账户/历史查询功能，新规划明确报告依赖未配置，不使用虚假行程替代。缺少 SMTP 时不能发注册/绑定验证码；HMAC 必须有效配置。

## 邮箱和 SMTP 配置

QQ 邮箱需要启用 SMTP 并使用生成的**授权码**，`SMTP_AUTH_CODE` 不是 QQ 登录密码。配置只放在本地环境中；下面均为占位值：

```dotenv
SMTP_HOST=smtp.qq.com
SMTP_PORT=465
SMTP_USERNAME=REPLACE_WITH_QQ_EMAIL_ADDRESS
SMTP_AUTH_CODE=REPLACE_WITH_QQ_SMTP_AUTH_CODE
MAIL_FROM=REPLACE_WITH_QQ_EMAIL_ADDRESS
SMTP_TIMEOUT_SECONDS=10
AUTH_CODE_HMAC_KEY=REPLACE_WITH_STABLE_RANDOM_HMAC_KEY
```

使用 `openssl rand -hex 32` 生成 HMAC 密钥并妥善保存；长度须为 32–512 字节，同一数据库的各实例及重启后必须保持一致，不能在每次启动时重新生成。`PLANNER_SERVICE_TOKEN` 应另行独立生成。SMTP 从建立连接时即使用 TLS（QQ 为 465），验证服务端证书，不支持 587/STARTTLS；自定义 TLS 端口供本地测试使用。`SMTP_TIMEOUT_SECONDS` 范围 1–10 秒，默认 10 秒。SMTP 部分配置缺失或无效会使 travel 启动失败；未注入任何 SMTP 配置时，发码返回 `MAIL_UNAVAILABLE`。

发码由 Travel RPC 处理，SMTP 不自动重试。成功后新 challenge 生效并替换同邮箱旧码，失败保留先前仍有效的码；超时后邮件仍可能实际送达，但该次码可能不可用。公开 API 只返回 `challenge_id/expires_in/retry_after`，从不返回验证码。已注册邮箱走同样的发邮件流程；持有效码重复注册返回 `EMAIL_ALREADY_REGISTERED`，消费该证明且不覆盖原密码。

新注册输入为 `email/password/code/challenge_id`，注册成功 201 返回 User，随后用 `email/password` 登录。密码使用 bcrypt，长度为 8–72 UTF-8 **字节**。邮箱去首尾空白、只小写域名，保留本地部分大小写及别名。旧账号用 `/api/v1/auth/legacy/login` 登录后，须调用 `/api/v1/me/email/code` 和 `/api/v1/me/email` 验证绑定邮箱；绑定前仅允许 `/me`、退出和绑定流程，其余受保护业务返回 403 `EMAIL_VERIFICATION_REQUIRED`。绑定沿用原 ID/密码/角色/行程和会话，之后只能用邮箱登录，当前版本不支持换绑。具体请求和错误处理见[前端接入指南](docs/frontend-integration.md)。

## 环境变量

| 变量 | 所属进程 | 说明 |
|---|---|---|
| DATABASE_URL | travel / admin | PostgreSQL 连接串，必填 |
| PLANNER_SERVICE_TOKEN | travel / planner | 内部服务认证，必填 |
| BIGMODEL_API_KEY | planner | 智谱凭据 |
| BIGMODEL_MODEL | planner | 默认 `glm-5.3-flash` |
| AMAP_API_KEY | travel | 高德 Web 服务类型凭据 |
| API_ADDR | api | 默认 `127.0.0.1:8080` |
| TRAVEL_ADDR | travel 监听 / api 连接 | 默认 `127.0.0.1:8888` |
| PLANNER_ADDR | planner 监听 / travel 连接 | 默认 `127.0.0.1:8889` |
| CORS_ORIGINS | api | 逗号分隔的前端来源白名单 |
| OPENAPI_PATH | api | 默认 `docs/openapi.yaml` |
| ADMIN_EMAIL | admin | 已完成邮箱验证的现有账号；仅提升角色，不创建账号或设置密码 |
| AUTH_CODE_HMAC_KEY | travel | 必填，32–512 字节；所有实例及重启保持一致 |
| SMTP_HOST / SMTP_PORT | travel | 默认 `smtp.qq.com` / `465`；建立连接即使用 TLS |
| SMTP_USERNAME / SMTP_AUTH_CODE / MAIL_FROM | travel | SMTP 账号、授权码及发件邮箱；启用邮件时完整填写 |
| SMTP_TIMEOUT_SECONDS | travel | 默认 10，范围 1–10 秒 |
| AUTH_CODE_TTL_SECONDS | travel | 默认 600，范围 60–900 秒 |
| AUTH_CODE_RESEND_SECONDS | travel | 默认 60，范围 30–300 秒 |
| AUTH_CODE_MAX_ATTEMPTS | travel | 同一有效 challenge 错码上限，默认 5，范围 1–10 |
| AUTH_SEND_EMAIL_HOUR / AUTH_SEND_IP_HOUR / AUTH_SEND_GLOBAL_HOUR | travel | 发码每小时邮箱/IP/全局限额，默认 5/100/500，范围分别为 1–50/1–10000/1–100000 |
| AUTH_VERIFY_EMAIL_HOUR / AUTH_VERIFY_IP_HOUR / AUTH_VERIFY_GLOBAL_HOUR | travel | 验证每小时邮箱/IP/全局限额，默认 30/300/3000，范围分别为 1–100/1–10000/1–100000 |
| AUTH_LOGIN_ACCOUNT_QUARTER / AUTH_LOGIN_IP_QUARTER / AUTH_LOGIN_GLOBAL_QUARTER | travel | 登录每 15 分钟账号/IP/全局限额，默认 10/100/1000，范围分别为 1–100/1–10000/1–100000 |
| ALLOW_LOCAL_PROVIDERS | 仅测试 | 显式设为 true 才允许回环 HTTP 测试端点 |
| AMAP_BASE_URL / BIGMODEL_BASE_URL | 仅测试 | 默认官方 HTTPS；只允许官方地址或已开启的回环测试地址 |
| SMTP_CA_FILE | 仅测试 | 本地 TLS SMTP 的测试 CA；仅在 `ALLOW_LOCAL_PROVIDERS=true` 且 SMTP_HOST 为 localhost/127.0.0.1 时可用，仍验证证书 |

示例文件不包含真实密钥。供应商 URL、认证字段与完整请求正文不写入应用日志。认证限流持久化在 PostgreSQL，多实例共享；429 `AUTH_RATE_LIMITED` 以响应正文 `retry_after` 和 `Retry-After` 头给出等待秒数。网关仅使用 TCP 对端 IP，不信任 `X-Forwarded-For` 等转发头；位于代理后时，每 IP 限额按代理连接地址聚合，当前没有可信代理配置入口。

## 验证

```sh
make test
make vet
# 先用 PostgreSQL 客户端创建独立数据库 travel_test、travel_e2e。
# 测试数据库不能运行规划执行器；避免测试任务被应用领取。
export TEST_DATABASE_URL='postgres://travel:local-travel-only@127.0.0.1:15432/travel_test?sslmode=disable'
make integration
make race
export TEST_DATABASE_URL='postgres://travel:local-travel-only@127.0.0.1:15432/travel_e2e?sslmode=disable'
make smoke
```

`make test` 在未设置 `TEST_DATABASE_URL` 时会跳过数据库测试，不能把这个结果当作数据库已验证。`make smoke` 使用真实 HTTP、两个真实 Kitex RPC 服务、PostgreSQL 和回环 TLS SMTP 接收器，模型/地图端点使用明确的本地 fixture，不向真实邮箱发测试邮件。它会启动并停止自己的应用进程，使用 18080/18081/18888/18889 及动态分配的回环 SMTP 端口，测试账户采用唯一邮箱，不删除数据。执行结果以[验证记录](docs/verification.md)为准。

如使用 SQL 客户端，连接开发数据库后分别执行 `CREATE DATABASE travel_test;` 和 `CREATE DATABASE travel_e2e;`，已有数据库无需重复创建。开发数据库工具不附带 SQL 客户端。

真实供应商联调需先按上文启动服务并配置 Key，并事先准备已经完成邮箱验证的账号。`--email` 必填；密码从 `TRAVEL_LOGIN_PASSWORD` 环境变量读取，未设置时在终端隐藏输入，不作为命令行参数传入。示例中的邮箱须替换为自己的已验证邮箱：

```sh
python3 scripts/live_smoke.py --email verified-user@example.com --days 3 --transport transit --revise
python3 scripts/live_smoke.py --email verified-user@example.com --days 3 --transport transit --revise --revision-scope day
python3 scripts/live_smoke.py --email verified-user@example.com --days 7 --transport transit
```

修改测试默认选择第二天下午；`--revision-scope day` 明确选择第二天整天，保留其他日期。较窄范围可能因边界交通费用变化返回 `SCOPE_CONFLICT`，测试不会绕过这个保护。命令使用已有账号创建真实规划任务、消耗供应商额度，不自动注册或发送验证码；脱敏结果保留在 `.local/live-*.json`。

## 代码结构与契约维护

```text
cmd/                 api、travel、planner、admin、devdb 启动入口
internal/domain/     业务模型、范围/行程校验、预算
internal/authn/      邮箱规范化、验证码 HMAC、认证策略
internal/mailer/     验证证书的 TLS SMTP 发信
internal/store/      PostgreSQL、会话、任务、版本及原子保存
internal/service/    业务接口、后台执行器、路线整合
internal/providers/  高德与 GLM 适配
internal/rpc/        Kitex 客户端、处理器、类型转换
internal/gateway/    Hertz 路由、请求校验与响应映射
idl/                 Thrift 契约
kitex_gen/           Kitex 0.16.3 生成代码
scripts/             契约生成及端到端验证
```

公共模型的维护入口是 `internal/domain/types.go`，`make generate` 生成同构 Thrift 模型及 Kitex 桩，RPC 接口名由 `scripts/generate_idl.py` 定义。生成器保留已有 Thrift 字段编号，新字段追加编号，并拒绝已有字段类型变化。生成文件不要手工修改。修改对外字段时同步更新 OpenAPI、前端文档并运行接口测试。

## 范围与估算说明

预算包含门票、餐饮、住宿、市内交通，不包含往返目的地的机票/火车票。餐饮初值按每人每餐 60 元估算；住宿按每两人一间、每间每晚 300 元估算，均有明确来源标签，非查询到的报价。门票无可信资料时保留未知；管理员可以补充费用来源、开放窗口和游玩时长。

公交优先在明确无公交方案时，可采用高德实查、不超过 1500 米且不超过 25 分钟的步行衔接，并返回实际方式和提示；其他地图故障不触发回退。

前端把 `constraints.transport` 当作偏好，并按每段 `route.mode` 渲染实际方式；公交优先回退成功的短步行会返回 `mode=walking`。

地图只验证已定位景点之间的衔接。餐厅、酒店和每日首尾起点尚未选定，所以每天额外保留一项未知市内接驳费用，`complete` 不会被误标为 true。地图耗时含查询时间，不能保证未来路况或班次。任务创建只持久化记录并快速返回 202；排队最多 300 秒，领取后执行最多 300 秒。模型单次非流式 HTTP 请求最多 120 秒，任务最多初次生成加一次修复。同步请求的参数、鉴权、冲突、依赖和超时分别由 HTTP 4xx/503/504 表达；任务返回 202 后的执行失败记录在任务 `status` 与 `error` 中，轮询任务本身仍返回 HTTP 200。

[架构与运行说明](docs/architecture.md)包含 RPC 边界、任务状态和故障行为。
