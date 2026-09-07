# AI 旅行行程规划师

这是一个 AI 旅行行程规划项目。前后端放在同一个仓库里：后端在 `BE/`，前端在 `FE/`，各自安装依赖、构建和运行。

仓库地址：<https://github.com/Chao-superw/ai-travel-planner>，目前为私有仓库。

```text
ai-travel-planner/
├── BE/                   Go 后端：Hertz 网关 + Kitex Travel/Planner RPC
│   ├── cmd/              服务和开发工具入口
│   ├── internal/         业务、持久化、外部适配与 HTTP/RPC
│   ├── idl/              Thrift 契约
│   ├── kitex_gen/        与当前 IDL 对应的生成代码
│   ├── docs/             OpenAPI、接入说明、架构与验证记录
│   ├── go.mod
│   └── go.sum
├── FE/                   React + TypeScript + Vite 前端
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── package-lock.json
├── .gitignore
└── README.md
```

## 从哪里开始

- [后端启动与环境配置](BE/README.md)
- [前端接入后端的约定](BE/docs/frontend-integration.md)
- [OpenAPI 接口契约](BE/docs/openapi.yaml)
- [HTTP 调用示例](BE/docs/examples.http)
- [后端验证记录](BE/docs/verification.md)
- [上传脱敏报告与本地保留文件清单](UPLOAD_GUIDE.md)

先按后端 README 在 `BE/` 中启动服务，默认 HTTP 地址是 `http://127.0.0.1:8080`。再启动前端：

```sh
cd FE
npm ci
# 首次配置时将 .env.example 复制为 .env.local，并填入本地配置。
npm run dev
```

前端通过 `VITE_API_BASE_URL` 指定 HTTP 网关，地图配置见 `FE/.env.example`。模型密钥和后端使用的高德 Web 服务密钥在后端运行环境中配置。

## 为什么采用一个仓库

接口和页面经常一起修改，放在同一个仓库里，可以在一次提交中更新后端接口、前端调用和文档。小组成员也能一次检出完整项目，复现课程演示。检查、构建和部署仍分别在 `BE/`、`FE/` 中进行。

以后如果前后端交给不同团队维护，需要不同的访问权限，或有独立的产品和发布周期，再考虑拆仓库。目前放在一起，省去了跨仓库同步接口版本的工作。

## 上传范围

仓库保留源码、静态资源、配置模板、文档、测试，以及 Thrift IDL 和对应生成代码。`go.mod`、`go.sum`、`package.json`、`package-lock.json` 也需要上传，其他人才能按同一套依赖安装。

根目录和子目录的 `.gitignore` 会排除真实 `.env` 配置、`BE/.local/` 中的数据库和运行记录、`FE/.local/` 中的启动记录，以及 `BE/bin/`、`FE/node_modules/`、前端构建产物和本机编辑器、工具缓存。协作者使用 `.env.example` 配置自己的环境，把 `REPLACE_WITH_*` 替换成实际值。真实值只填在本地配置里，不写回示例文件。

前后端目前共用高德 Key，`AMAP_API_KEY` 与 `VITE_AMAP_KEY` 填写同一值。源码引用环境变量，模板统一写成 `REPLACE_WITH_SHARED_AMAP_KEY`。前端构建后，这个值会出现在浏览器代码中。脱敏处理、检查范围和本地保留文件见[上传脱敏报告](UPLOAD_GUIDE.md)。

## 协作约定

- 只在项目根目录维护一个 `.git`；`BE/` 和 `FE/` 都属于同一个仓库。
- `main` 保存前后端完整代码。开发分支可命名为 `feat/be-具体功能`、`feat/fe-具体功能`，也可以按跨端功能命名，完成后通过 PR 合入。
- 修改接口时，同时更新 `BE/docs/openapi.yaml` 和前端调用，尽量放在同一个 PR 中审阅。HTTP 接口约定以这份 OpenAPI 文件为准，前端无需再维护一份副本。
- 首次上传前先看 `git status` 和待跟踪文件。可以按仓库说明、后端、前端分组提交，方便分工和审阅，最后推送到同一个 `main`。
- 后端检查在 `BE/` 中执行，前端使用 `FE/package.json` 中的 `test`、`lint`、`build`。创建 GitHub 仓库不会自动运行这些检查，CI 的执行结果需要另行确认。

参考：[GitHub 仓库创建说明](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository)、[GitHub 忽略文件说明](https://docs.github.com/en/get-started/git-basics/ignoring-files)。
