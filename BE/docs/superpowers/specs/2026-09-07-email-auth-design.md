# 邮箱注册与登录设计

已授权范围：只开发 BE 和前端接入文档；QQ SMTP 配置已在本地完成，用户明确要求开始实现。不提交、推送或删除现有文件，不改 FE。

## 行为与边界
- 新注册：发送注册验证码，再提交 email/password/code/challenge_id；成功 201，随后邮箱密码登录，复用 Bearer 会话。
- 邮箱用 ASCII addr-spec，拒绝显示名、控制字符和多地址；去首尾空格、域名小写，保留本地部分大小写，不做 QQ 别名或 Gmail 点号合并。
- 验证码 6 位安全随机数，默认有效 600 秒、重发间隔 60 秒、每邮箱每小时 5 次、每 IP 每小时 100 次、全局每小时 500 次。错误最多 5 次，跨 challenge 的猜测也受邮箱和 IP 限制。配置有上下界。
- 邮箱挑战包含随机 ID、邮箱键、purpose(register/bind)、绑定用户 ID、HMAC、发送状态、过期时间、错误次数、消费时间。HMAC 密钥至少 32 字节，本地自动生成，其他环境显式配置。
- 同一邮箱串行申请；发送成功才令新码有效并替代旧码；失败保留旧码。发送有独立超时，不在数据库事务中等待 SMTP；失败和未完成状态不能用于注册。SMTP 不自动重试。
- 发送端对已注册邮箱同样发送验证邮件；只有验证码持有人提交注册后才收到 EMAIL_ALREADY_REGISTERED，因此不使用账户存在性决定公开发码响应。邮件文案说明已注册用户应登录，验证码不会修改已有密码。
- 注册/绑定时锁邮箱及 challenge，核对用途、邮箱、所属用户、有效期和次数；创建或绑定与消费码原子提交。错误尝试计数独立提交，不能被错误回滚。邮箱唯一索引兜底并发。
- 旧账号 ID、行程、角色、密码哈希保留；LegacyLogin 只接受尚未绑定邮箱的账号，绑定前只允许 me/logout/发绑定码/绑定邮箱。绑定验证后原 ID 与会话继续使用，旧用户名登录失效。旧管理员按同样方式迁移。管理员命令只提升已验证邮箱用户，不创建未验证特权账号。
- 新账号展示名为服务端生成的旅行者名称，username 字段作为兼容展示字段保留，不作为新登录输入。用户响应追加 email 和 email_verified。
- 密码沿用 bcrypt，8–72 字节，不截断；邮箱登录失败使用相同错误和 dummy bcrypt 验证路径，数据库故障单独报告。登录请求按账号、IP 及全局限流。
- Travel RPC 负责全部认证、数据库和 SMTP；网关仅接受窄 DTO，并从连接远端地址读取 IP，不信任外部 X-Forwarded-For。RPC 端口继续仅内部网络；发送超时 SMTP 10 秒、RPC 14 秒、HTTP 15 秒。
- SMTP 使用 TLS 和证书验证，邮件纯文本 MIME，授权码和验证码不进入日志/公开 API。测试只使用本地 TLS SMTP 接收器；不向真实邮箱发测试消息。

## HTTP 契约
POST /api/v1/auth/register/code {email} → 200 {challenge_id,expires_in,retry_after}
POST /api/v1/auth/register {email,password,code,challenge_id} → 201 User
POST /api/v1/auth/login {email,password} → 200 Session
POST /api/v1/auth/legacy/login {username,password} → 200 Session（仅迁移）
POST /api/v1/me/email/code {email} + Bearer → 200 Challenge
POST /api/v1/me/email {email,code,challenge_id} + Bearer → 200 User
GET /api/v1/me 和 POST /api/v1/auth/logout 保持现有路径。
400 INVALID_INPUT；401 INVALID_CREDENTIALS；403 EMAIL_VERIFICATION_REQUIRED；400 INVALID_VERIFICATION_CODE（含过期/重放/错误用途）；409 EMAIL_ALREADY_REGISTERED/EMAIL_ALREADY_BOUND；429 AUTH_RATE_LIMITED 带 retry_after；503 MAIL_UNAVAILABLE/DATABASE_UNAVAILABLE。

## 迁移、运行和验收
PostgreSQL 增量添加 users 邮箱列/唯一索引，新增 auth_challenges 与 auth_rate_limits；无清库和删表。IDL 保留现有编号并追加字段/RPC方法，生成器读取已有编号且拒绝既有字段类型变化。
真实 PostgreSQL 测试覆盖重放、重发、过期、错码次数、邮件失败、并发、事务回滚、迁移和重启；TLS SMTP 本地协议测试覆盖认证、邮件内容、拒收和超时；HTTP→Kitex→PG→本地SMTP完整 smoke。更新 OpenAPI、README、examples.http、前端接入文档和验证日志。
