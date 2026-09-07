# GitHub 上传脱敏报告

检查日期：2026-09-07。适用范围：同一仓库中的 `BE/` 与 `FE/`。

前后端继续共用高德 Key。仓库只保存源码和配置模板，真实凭据留在本地环境中。本报告记录首次提交前的脱敏检查：检查过程中没有删除本地配置、数据库或构建产物，也没有执行提交、推送或远端设置变更。后续发布状态以 Git 历史为准。

## 1. 密钥与个人信息的处理

业务代码保留环境变量引用，不把占位符写死进业务逻辑。下面的占位符放在 `.env.example` 和配置说明中，协作者在自己的本地配置中替换。**`REPLACE_WITH_*` 不是可用凭据，启动前必须替换，真实值不要写回示例文件。**

| 配置项 | 上传时的明确占位符 | 实际配置位置 |
|---|---|---|
| `BIGMODEL_API_KEY` | `REPLACE_WITH_BIGMODEL_API_KEY` | 后端本地环境 / `BE/.env` |
| `AMAP_API_KEY`、`VITE_AMAP_KEY` | `REPLACE_WITH_SHARED_AMAP_KEY` | 后端本地环境与 `FE/.env.local`，两者填写同一值 |
| `PLANNER_SERVICE_TOKEN` | `REPLACE_WITH_RANDOM_SERVICE_TOKEN` | 后端本地环境，Travel 与 Planner 保持一致 |
| `AUTH_CODE_HMAC_KEY` | `REPLACE_WITH_STABLE_RANDOM_HMAC_KEY` | `BE/.env` / 后端环境，重启和多实例间保持稳定 |
| `SMTP_USERNAME`、`MAIL_FROM` | `REPLACE_WITH_QQ_EMAIL_ADDRESS` | `BE/.env`，填写完整的发信 QQ 邮箱 |
| `SMTP_AUTH_CODE` | `REPLACE_WITH_QQ_SMTP_AUTH_CODE` | `BE/.env`，填写 SMTP 授权码 |
| `POSTGRES_PASSWORD`、`DATABASE_URL` 中的密码 | `REPLACE_WITH_DATABASE_PASSWORD` | 本地配置，必须与实际数据库账户一致 |

已处理的文件：

- `BE/.env.example`、`FE/.env.example`：将空白或旧式示例统一为明确占位符，高德使用相同的占位符名称。
- `BE/internal/authn/email_test.go`：真实邮箱替换为测试地址 `traveler@example.test`，保留邮箱规范化的测试逻辑。
- `BE/docs/gateway-task.md`、`BE/docs/provider-task.md`：本机绝对路径替换为相对仓库的 `BE/`。
- 三份 `README.md`：补充占位符、本地配置和上传范围说明。
- 根 `.gitignore`：显式列出前端运行目录，并补充私钥、数据库导出及网络抓包文件的排除规则。

代码测试中的虚构密码、测试令牌和验证码用于验证行为，不是实际凭据，保留以保证测试可复现。`make dev-db` 与 Compose 原有回退配置使用公开的本地演示口令 `local-travel-only`，它也不是脱敏遗漏；部署其他环境须填写独立的数据库密码。模板已改为数据库密码占位符。修改配置不会自动修改已有数据库账户的密码。

### 共用高德 Key 的说明

当前 `AMAP_API_KEY` 与 `VITE_AMAP_KEY` 使用同一真实值，本次保留该决定。后端查询景点和交通路线，前端加载地图 JS API；没有在本次调整中拆分 Key、增加代理或改变供应商配置。

前端 `VITE_AMAP_KEY` 会被写入构建后的 JavaScript，地图脚本请求也会携带该值。当前本地 `FE/dist/` 的 JavaScript 中已能匹配到此 Key，因此源码中的占位符和 Git 忽略规则只解决源码上传问题，**不能让部署后的浏览器 Key 保密**。共用时，前端可见的就是后端使用的同一 Key；本次未更改或验证服务商白名单、配额和权限设置。该行为与 [Vite 环境变量说明](https://vite.dev/guide/env-and-mode)一致。

SMTP 授权码、模型 Key、服务间令牌和 HMAC 密钥继续只由后端读取，不进入 `VITE_*`。共用地图 Key 不代表将其他后端凭据交给浏览器。

## 2. 留在本地、不上传的文件

以下已有内容保留原位，通过 Git 忽略规则排除：

| 路径或类型 | 留在本地的原因 |
|---|---|
| `BE/.env`、`FE/.env.local`、其他 `.env` / `.env.*`（除 `.env.example`） | 实际服务配置、邮箱、授权码和密钥 |
| `BE/.local/` 整个目录 | PostgreSQL 数据、用户邮箱/密码哈希/会话/行程、日志、PID、联调输出、旧文件快照、诊断与临时脚本、测试证书、下载的数据库工具 |
| `FE/.local/` 整个目录 | 本机 Vite 启动信息、进程记录和日志 |
| `BE/bin/` | 本机构建的可执行文件；上传源码后重新构建 |
| `FE/dist/`、`dist-ssr/` | 前端构建产物，可能已包含真实地图 Key |
| `node_modules/` | 本机安装的依赖；通过锁文件重新安装 |
| `coverage/`、`*.tsbuildinfo`、`__pycache__/`、`*.pyc` | 测试、编译和解释器缓存 |
| `.trae/`、`.sdd-fe/`、`.superpowers/`、`.codex/`（根目录） | 本机 AI 工具状态和会话材料，可能带有路径或原始输入 |
| `.idea/`、`.DS_Store`、`*.log`；`FE/.vscode/` 中除 `extensions.json` 外的文件 | 本机编辑器、系统状态和日志 |

对以后新增内容，根 `.gitignore` 还排除了 `secrets/`、`*.key`、`*.pem`、`*.p12`、`*.pfx`、`*.dump`、`*.sql.gz`、`*.har`。原始 SQL 数据备份、含账号/Key 的截图、抓包、认证导出、原始对话和临时压缩包统一放进 `BE/.local/` 或 `FE/.local/`，不要散落在源码或文档目录。这里的规则也会排除这些扩展名的测试证书文件；需要提交测试材料时优先像现有 SMTP fixture 一样在测试时临时生成。

不全局忽略 `*.sql`、`*.json` 或图片：数据库表结构、迁移脚本、项目配置和前端素材可能正是运行项目所需的源码。

## 3. 应上传的文件

| 内容 | 保留的用途 |
|---|---|
| `BE/cmd/`、`BE/internal/`、`BE/idl/`、`BE/kitex_gen/` | 后端源码、测试、数据库迁移、RPC 契约及对应生成代码 |
| `BE/go.mod`、`BE/go.sum`、`BE/Dockerfile`、`BE/compose.yaml`、`BE/Makefile`、`BE/scripts/` | 依赖、构建与验证入口；脚本从运行环境获取凭据 |
| `FE/src/`、`FE/public/`、`FE/index.html`、构建/测试配置 | 前端源码、静态资源与构建方式 |
| `FE/package.json`、`FE/package-lock.json` | 可复现的前端依赖安装 |
| `BE/.env.example`、`FE/.env.example`、`.gitignore`、`.dockerignore` | 不含真实凭据的配置模板与排除规则 |
| `README.md`、`UPLOAD_GUIDE.md`、`BE/docs/`、`FE/README.md` | 启动方法、前端接口契约、架构、验证结果、Vibe Coding 迭代及设计记录 |

`BE/docs/superpowers/` 是项目设计和开发计划，作为课程过程材料保留；它与根目录 `.superpowers/` 的本机工具状态不同。历史任务/评审记录也保留，当前接口以 `BE/docs/openapi.yaml` 和 `BE/docs/frontend-integration.md` 为准。

本次查看了候选文件中的 14 张图片：`BE/docs/examples/` 的 7 张设计图、`FE/public/templates/` 的 6 张城市图、`FE/src/assets/hero.png`。可见内容未发现 Key、验证码或真实账号截图；新增图片仍需单独检查。

## 4. 首次提交前的脱敏检查

本次使用 `git ls-files --cached --others --exclude-standard -z` 获取全部候选文件，不仅搜索已跟踪源码。结果如下：

| 检查 | 结果 |
|---|---|
| 候选文件 | 191 个：177 个文本文件、14 张图片 |
| 已知凭据比对 | 从两份本地环境文件和已确认身份的项目 Travel/Planner 进程读取 8 个变量，逐文件比对原值及 URL 编码形式；模型 Key 也比对组成部分。0 处命中 |
| 常见密钥格式检查 | 私钥块、GitHub token、智谱 Key 格式、Key/Token/Secret 后的 32 位十六进制字面值及 JWT 格式，0 处命中；只输出类型和位置，不输出凭据 |
| 本机项目绝对路径 | 候选文件中 0 处命中 |
| 忽略规则 | 19 个现有或代表性路径全部被排除；两份 `.env.example` 仍在候选清单中 |
| 配置模板 | 所有上述敏感配置均使用明确占位符；前后端地图占位符一致 |
| 本地高德 Key | 实际前后端值仍然相同；本地被忽略的 `FE/dist/` 中仍有该值，未上传、未删改 |
| 后端测试 | `GOCACHE=/tmp/ai-travel-build GOPROXY=off go test ./internal/authn` 通过；覆盖本次改动的邮箱测试样本 |
| 脱敏检查时的 Git 状态 | 0 个已跟踪文件，无本地提交；这次脱敏操作未暂存、提交或推送 |

本次没有写入 `BE/.env` 或 `FE/.env.local`。前后 SHA-256 比对确认 `BE/.env` 内容不变；检查期间 `FE/.env.local` 的 `VITE_API_BASE_URL` 从本机网关地址变为空值，文件哈希因此变化。仅将这一项还原后计算得到的哈希与检查开始时完全一致，确认地图 Key 及其他字节未变。本次保留该同时发生的本地配置修改，没有覆盖接口地址，也没有重启服务。

检查结论只覆盖这次候选文件、已知值比对、所列格式规则和图片可见内容；没有运行 Gitleaks，没有验证或开启 GitHub 推送保护，也不把零命中描述为对任意未知/编码/嵌入秘密的绝对保证。本次改动不涉及业务逻辑，未重复执行整套前端构建、数据库或真实供应商联调；此前功能验证记录另见 `BE/docs/verification.md` 与 `FE/README.md`。

## 5. 后续上传或交付

在仓库根目录用以下只读命令查看实际候选文件及规则：

```sh
git status --short --untracked-files=all
git ls-files --cached --others --exclude-standard
git check-ignore -v BE/.env FE/.env.local BE/.local FE/.local FE/dist FE/node_modules BE/bin
```

`.gitignore` 只约束未被跟踪文件的默认加入行为；不要使用 `git add -f` 强行加入敏感内容。未来文件已被跟踪时，添加忽略规则不能清除提交中的内容。暂存之后还要检查暂存文件清单和差异，确认实际提交的内容与检查过的文件一致。

**GitHub 网页直接上传文件、Finder 压缩整个目录、手工创建 Release 附件，都不会自动按 `.gitignore` 筛选。** 课程源码压缩包也只应包含第 3 节的源文件，并在发出前核对包内目录，不能直接打包当前工作区。根 `.git/` 是本地版本管理元数据，不要删除，也不要作为源码附件上传。

协作者拿到源码后，按 README 安装依赖，在自己的本地环境填写配置并重新构建。源码仓库不分发个人真实凭据、测试账户数据或现成的本机构建包。

## 6. 首次推送前复查

仓库首页和提交说明按 humanizer 整理措辞，保留技术事实。远端检查确认目标仓库为私有仓库，尚无分支或提交。

后端运行 `go test ./...` 通过。这次没有设置 `TEST_DATABASE_URL`，需要数据库的测试会跳过，因此不能用这次结果代替数据库集成测试记录。

前端复查发现认证测试的请求替身只接受完整 URL，同源 `/api` 请求会被误报为网络失败。已修正测试的解析方式，并分别验证相对地址和完整地址下的注册、登录流程。修正后 46 项测试通过，lint 没有错误、保留 10 条已有告警，TypeScript 和生产构建通过。构建输出放在被忽略的 `FE/.local/` 中，原有 `FE/dist/` 和本地配置保留原位。

暂存区共有 191 个文件，按实际暂存内容复查后，已知凭据和所列密钥格式均为零命中。本地配置、数据库、日志和构建产物没有进入暂存区。提交使用 GitHub noreply 邮箱，避免把本机配置的个人邮箱写入提交元数据；推送结果以远端 `main` 与本地提交的比对为准。
