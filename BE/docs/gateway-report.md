# Gateway 实现报告

## 范围

- `internal/gateway`：Hertz 路由、严格输入解析、Bearer 前置校验、RPC 调用映射、响应封装、超时、CORS、健康检查与 OpenAPI 文件读取。
- `cmd/api/main.go`：按约定环境变量创建 Travel RPC 客户端并启动 API。
- `docs/openapi.yaml`：系统端点和全部 17 个 `/api/v1` 用户端点的 OpenAPI 3.0.3 契约。
- `docs/frontend-integration.md`、`docs/examples.http`：登录、任务轮询、版本冲突、窄化编辑、预算未知项、GCJ-02 与请求示例。

## 行为边界

- 每次请求由网关生成独立 `request_id`；受保护端点缺少 Bearer 时不会调用 RPC。
- JSON 最大 1 MiB，拒绝未知字段和尾随 JSON。查询整数拒绝负数、显式 0、溢出和超上限值。
- 通用调用超时 5 秒，地点搜索/创建/更新为 8 秒。RPC 传输异常只返回脱敏 503；上下文截止和 Kitex 原生 RPC 超时均返回脱敏 504。
- 配置源之外不返回 CORS 许可；配置源预检返回 204。允许 `Authorization`、`Content-Type`、`Idempotency-Key`。
- 行程和版本列表只输出摘要。任务接受响应附带 `status_url`。
- 手动编辑输入不接受地点快照、费用、路线等服务端字段；地点更新不接受供应商身份、名称或坐标。

## 验证

执行：

```text
GOCACHE=/tmp/ai-travel-build GOMODCACHE=/tmp/ai-travel-mod go test ./internal/gateway ./cmd/api
```

结果：`internal/gateway` 通过；`cmd/api` 编译通过（无测试文件）。另使用 Ruby Psych 解析 `docs/openapi.yaml`，确认 YAML 语法有效。

## 规划超时初值校准

根据真实 7 日 GLM 非流式首次调用超过原 60 秒 HTTP 上限的现象，统一初值为：模型 HTTP 120 秒、Planner RPC 130 秒、任务执行 300 秒。任务仍最多容纳首次生成与一次修复，并为校验、算路和持久化保留 60 秒预算；排队上限仍为 300 秒，租约 30 秒、续租周期 10 秒均未调整。真实 smoke 的轮询上限调整为 650 秒，以覆盖排队和运行两个阶段。

本次只执行本地单元测试、编译检查与 smoke 脚本语法检查，不调用真实模型；因此不宣称真实供应商端到端已通过。另提供不等待 300 秒的真实 Claim 测试，用于核对 300 秒截止时间、5 分钟排队边界和 30 秒租约；当前本机测试数据库端口不可连接，该数据库用例尚未实际通过。
