# AI Travel Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 交付可运行的 Kitex 三进程旅行规划后端及完整前端接入文档。

**Architecture:** Hertz gateway 仅负责 HTTP，Travel RPC 独占 PostgreSQL 与高德适配，Planner RPC 独占 GLM 适配。规划任务持久化，数据库事务不包含外部网络调用，成功时原子保存不可变版本及路线。

**Tech Stack:** Go、Kitex/Thrift TTHeader、Hertz、pgx/PostgreSQL；版本在 go.mod 锁定。

**Spec:** ../specs/2026-09-07-ai-travel-backend-design.md

## Global Constraints

- 三个应用进程；单城市；表单创建、自然语言局部修改；全团人民币分。
- 身份及所有权在 travel 验证；模型最多两次调用；地图结果不得伪造。
- 所有任务提交含 Idempotency-Key；编辑以 expected_version 做乐观并发控制。
- 密钥只读取运行环境，不进入源码、示例、日志或前端响应。
- 用户已要求直接完成后端及文档；使用已讨论的默认范围，无需再次询问排期。
- 当前目录是专用新项目目录，没有 Git 仓库；原地开发，不创建提交、不推送、不删除文件。

## File Structure

`cmd/{api,travel,planner,admin}` 为启动入口；`idl/{model,travel,planner}.thrift` 为契约；`kitex_gen/` 为生成代码；`internal/domain/` 为业务模型及纯校验；`internal/store/` 为 PostgreSQL；`internal/providers/` 为外部适配；`internal/service/` 为业务和任务；`internal/rpc/` 为强类型 RPC 转换；`internal/gateway/` 为 HTTP；`docs/` 为 OpenAPI 与接入文档；`scripts/` 为验证及运行工具。

### Task 1: 契约及业务校验

**Files:** go.mod、idl/*.thrift、internal/domain/{types,validate,budget}.go 及 *_test.go、internal/config/config.go。
**Interfaces:** `domain.Constraints`, `Place`, `Activity`, `Plan`, `Scope`, `Route`, `JobInput`, `Job`, `User`, `APIError`；`ValidateConstraints(*Constraints) error`、`ValidatePlan(Constraints, Plan, []Place) []string`、`MergeRevision(Plan, Scope, []Activity) (Plan,error)`、`Budget(Plan,int) BudgetSummary`。

- [x] 写表驱动测试：8 天拒绝、全团/人均换算、日期错误、重复/虚构 POI、重叠活动、范围外修改、未知费用不作零预算。
```go
if err := ValidateConstraints(&Constraints{City:"杭州市",StartDate:"2026-10-01",EndDate:"2026-10-08",PartySize:1,BudgetCents:100000,BudgetScope:"total",Transport:"walking"}); err == nil { t.Fatal("accepted eight days") }
```
- [x] `go test ./internal/domain` 观察缺失实现失败，补齐结构及校验后运行通过。
- [x] 按公共结构定义 Thrift 请求与响应，使用锁定版本 kitex 生成客户端和服务端代码；不手写序列化代码。
```sh
go run github.com/cloudwego/kitex/tool/cmd/kitex -module ai-travel idl/travel.thrift
go run github.com/cloudwego/kitex/tool/cmd/kitex -module ai-travel idl/planner.thrift
```

### Task 2: 存储、身份和可靠任务

**Files:** internal/store/{schema.sql,store.go,auth.go,jobs.go,trips.go,places.go}、internal/service/{travel,worker}.go、cmd/admin/main.go；集成测试放 internal/store 与 internal/service。
**Interfaces:** `store.Open(ctx,url) (*Store,error)`、`Migrate(ctx) error`；服务方法以 domain 请求/结果为边界；用户身份从会话解析；任务执行器依赖 MapProvider 与 Planner 接口，不让 gateway 持有数据库。

- [x] 建立真实 PostgreSQL 测试数据库；写登录撤销、跨用户不可见、幂等竞争、版本竞争与租约到期测试，先观察失败。
```sql
SELECT id FROM planning_jobs WHERE status='queued' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1;
UPDATE trips SET current_version=current_version+1 WHERE id=$1 AND current_version=$2;
```
- [x] 用用户行锁串行化配额与幂等插入；原子保存版本、活动、路线、成功状态。模型/地图调用在事务外。
- [x] 执行任务续租、取消、绝对截止时间、过期恢复和显式重试；注入失败后确认无半个版本、无迟到覆盖。
```sh
go test ./internal/store ./internal/service -count=1
```

### Task 3: 外部服务及 Kitex RPC

**Files:** internal/providers/{amap,glm}.go 与 *_test.go；internal/rpc/；cmd/{travel,planner}/main.go。
**Interfaces:** `MapProvider.Search(ctx,city,keyword string,page,size int) ([]Place,bool,error)`、`MapProvider.Route(ctx,from,to Place,mode string) (Route,error)`；`Planner.Generate(ctx,PlanningRequest) (PlanningResult,error)`、`Planner.Revise(ctx,PlanningRequest) (PlanningResult,error)`。

- [x] 对 httptest 供应商边界先写业务错误、限流、超时、非法 JSON、截断输出、空路径测试。请求必须真正经过适配器，断言行为与参数。
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ w.Header().Set("Content-Type","application/json"); fmt.Fprint(w,`{"status":"0","infocode":"10003","info":"DAILY_QUERY_OVER_LIMIT"}`) }))
defer server.Close()
```
- [x] 实现城市/类别筛选、路线耗时及费用解析、凭据脱敏、有限响应体、无隐式生成重试；GLM 最多每个 RPC 一次调用。
- [x] 接入 TTHeader 及 deadline 传播，服务凭据认证；运行真实三进程回环测试。
- [x] 对用户指定模型做官方最小调用；不支持时如实记录，不替换模型。

### Task 4: HTTP 与前端契约

**Files:** internal/gateway/、cmd/api/main.go、docs/openapi.yaml、docs/frontend-integration.md、docs/examples.http。
**Interfaces:** HTTP /api/v1 与 spec 列表一致；成功 `{data,request_id}`，错误 `{code,message,request_id,field_errors?}`；snake_case；任务 202；分页 `{items,page,page_size,has_more}`。

- [x] 先写路由边界测试：无效 JSON、未知字段、缺幂等键、认证、错误映射、允许的 CORS 来源和前端任务轮询流程。
```sh
go test ./internal/gateway -count=1
```
- [x] 实现所有 spec API，正文 1 MiB 限制，RPC 参数转强类型，严格分页；GET /healthz、GET /readyz、GET /openapi.yaml。
- [x] 提供完整字段、空值、错误码、局部范围构造、版本冲突处理和预算图表数据示例，前端只对接 HTTP。

### Task 5: 运行、整体验证及交付

**Files:** Dockerfile、compose.yaml、.env.example、.gitignore、Makefile、README.md、scripts/smoke.py、docs/verification.md、docs/architecture.md。

- [x] Build 三个独立二进制及管理员工具；提供 PostgreSQL Compose 持久卷和健康检查，不暴露 RPC 端口到公网。
```sh
go build ./cmd/...
go test -race ./internal/...
go vet ./...
```
- [x] 真 PostgreSQL + 真 Kitex + 真 Hertz + 受控模型/地图 HTTP 服务跑端到端：注册→生成→轮询→预算→局部修改→新版本→跨用户拒绝→撤销登录；进程重启后仍可读。
- [x] 分别记录模拟依赖与真实供应商结果；执行真实地图与指定 GLM 联调可行部分，失败保持真实错误。
- [x] 独立代码评审，修正阻断问题并复验；README 给出最短启动及前端接入路径。

## Execution Record

进度、测试和实现差异记录于 `docs/verification.md`；不使用 Git 提交作为进度载体。

最终验收：本机及 Linux 编译、真实 PostgreSQL 全仓 race/vet、11 项受控依赖 E2E 均通过；真实高德与 GLM v6 的三日/七日生成、第二天整日重排到版本 2 通过。下午窄范围的实际费用边界冲突如实记录；未安装 Docker，Compose 仅配置与 YAML 验证，未声称容器实际运行。
