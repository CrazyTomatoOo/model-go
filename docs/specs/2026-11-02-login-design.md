# 登录功能实现 Spec

日期：2026-11-02 · 状态：初稿（待评审） · 关联：issue #6（wayfinder 地图 #1 终点）、ADR-0001

## 1. 目标与范围

为 console 提供本地账号认证：登录、登出、改密、会话管理、管理员账号生命周期，对齐等保三级（三权分立、登录失败处理、审计留存 ≥180 天）。

**范围内**：`cmd/apiserver` 的认证 API、`admin bootstrap` CLI 子命令、users/audit_logs 两张表、Redis 会话、认证/CSRF/角色中间件、登录失败锁定。

**范围外**：console 前端 UI（`web/apps`，以后对着本 spec 的 API 实现）；OIDC/LDAP/JWT；agent-gateway 的 mTLS 认证（独立 spec）；密码找回（无自助找回，忘记密码 = 管理员重置）。

## 2. 技术选型（均已有决议）

| 项 | 选型 | 决议出处 |
|---|---|---|
| HTTP 框架 | gin v1.10.x（go 1.24 工具链，pin 不升 v1.12） | ADR-0001 + m00348 |
| 密码哈希 | argon2id，PHC 字符串存储 | #2，m00371 |
| 会话 | 服务端 Redis 会话 + `__Host-` cookie | #3，m00358 |
| 账号存储 | Postgres（平台已有选型） | ADR-0001 |
| 分权 | 三权分立，账号双人开通 | #5，m00366 |

新增依赖：`github.com/gin-gonic/gin v1.10.1`、`github.com/alexedwards/argon2id`（PHC 编解码）、`github.com/redis/go-redis/v9`、`github.com/jackc/pgx/v5`、`github.com/golang-migrate/migrate/v4`（迁移）。

## 3. 数据模型

### 3.1 Postgres

```sql
CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  username      TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,              -- PHC: $argon2id$v=19$m=65536,t=3,p=4$...
  role          TEXT NOT NULL CHECK (role IN ('user','sysadmin','secadmin','auditadmin')),
  status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
  must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
  created_by    BIGINT REFERENCES users(id),
  approved_by   BIGINT REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_login_at TIMESTAMPTZ
);

CREATE TABLE audit_logs (
  id             BIGSERIAL PRIMARY KEY,
  actor_id       BIGINT,                    -- 未登录时为 NULL
  actor_username TEXT NOT NULL,
  actor_role     TEXT,
  action         TEXT NOT NULL,             -- 见 §7 动作清单
  target         TEXT,                      -- 如 users:42 / sess:xxx
  detail         JSONB,
  ip             INET,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_created ON audit_logs(created_at);
CREATE INDEX idx_audit_actor ON audit_logs(actor_id);

-- 历史密码（§6 密码策略：禁止与最近 5 次历史密码重用）
CREATE TABLE password_history (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id),
  password_hash TEXT NOT NULL,            -- 改密前的旧密码 PHC（当前密码不在此表）
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_password_history_user ON password_history(user_id, created_at DESC);
```

审计表约束：**append-only**。应用 DB 角色对 audit_logs 只授 INSERT/SELECT(REVOKE UPDATE/DELETE),物理删除只能由 DBA 在到期后执行。留存 ≥180 天。

### 3.2 Redis key

| key | 类型 | 内容 | TTL |
|---|---|---|---|
| `sess:{sid}` | hash | `user_id, csrf, created_at, abs_expires`（不含敏感数据） | 30min，活动 touch |
| `lock:{username}` | string | 连续失败计数 | 15min，每次失败重置 |

`sid` = 256-bit CSPRNG 不透明串（hex 或 base64url，64 字符内）。

## 4. API 契约

统一约定：
- 所有请求/响应体为 JSON；路径前缀 `/api/v1`。
- 认证：cookie `__Host-session`（HttpOnly、Secure、SameSite=Lax、Path=/、无 Domain）。
- 除 `POST /auth/login` 外，所有非 GET 请求须带 `X-CSRF-Token: <csrf_token>`（值来自 `GET /auth/me` 响应或登录响应），与会话内值不一致 → 403。
- 改密触发后（`must_change_password=true`），仅放行 `GET /auth/me` 与 `POST /auth/password`，其余一律 403 `PASSWORD_CHANGE_REQUIRED`。

### 4.1 认证

| 方法/路径 | 角色 | 请求 → 响应 |
|---|---|---|
| `POST /auth/login` | 匿名 | `{username, password}` → 200 `{user:{id,username,role,status,must_change_password}, csrf_token}` + Set-Cookie；失败一律 401 `{code:"INVALID_CREDENTIALS"}`（文案统一「用户名或密码错误」，不区分不存在/密码错/停用/待审核/锁定） |
| `POST /auth/logout` | 登录 | → 204，删会话、清 cookie |
| `GET /auth/me` | 登录 | → 200 `{user, csrf_token}` |
| `POST /auth/password` | 登录 | `{old_password, new_password}` → 204；清 `must_change_password`、踢掉该用户**其他**会话（保留当前） |

登录成功后的状态检查（密码已验证通过，才返回具体原因）：
- `status=pending` → 403 `ACCOUNT_PENDING`「账号待安全管理员审核启用」
- `status=disabled` → 403 `ACCOUNT_DISABLED`「账号已停用，请联系管理员」
- 正常 → 建会话（防固定攻击：绝不复用已有 sid）、写审计 `auth.login_success`、更新 `last_login_at`

失败处理：写审计 `auth.login_failure`；`lock:{username}` 计数 +1，达到 5 → 锁 15 分钟，锁定期内即使密码正确也返回统一 401。

登录接口限流：每 IP 每分钟 20 次 → 429 `RATE_LIMITED`（Redis 计数）。

### 4.2 账号管理（sysadmin）

| 方法/路径 | 请求 → 响应 |
|---|---|
| `POST /users` | `{username, password, role}` → 201 `{id, status:"pending"}`。用户名唯一（冲突 409）。`must_change_password=true` 由服务端强制写入，**请求体不接受该字段** |
| `PATCH /users/:id/password` | `{password}` → 204。重置：置 `must_change_password=true`、踢该用户全部会话；**不改变 status**（重置 ≠ 重新开通） |
| `POST /users/:id/disable` | → 204。status=disabled、踢全部会话 |

### 4.3 安全操作（secadmin）

| 方法/路径 | 说明 |
|---|---|
| `POST /users/:id/approve` | pending → active，记 `approved_by`；停用账号重新启用也走此接口（双人复核防单管理员复活账号） |
| `POST /users/:id/unlock` | 清除登录锁定 |
| `GET /sessions?user_id=&page=` | 会话枚举（返回 sid 摘要、创建时间、绝对过期时间，不含 csrf） |
| `DELETE /sessions/:sid` | 强制下线 |

### 4.4 审计查询（auditadmin）

`GET /audit-logs?actor_id=&action=&from=&to=&page=` → 分页返回。**只读**，该角色无任何管理接口。

### 4.5 错误码表

| HTTP | code | 含义 |
|---|---|---|
| 400 | VALIDATION_ERROR | 字段校验失败（附字段级明细） |
| 401 | INVALID_CREDENTIALS / SESSION_EXPIRED | 凭据失败（统一文案）/ 会话缺失或过期 |
| 403 | ACCOUNT_PENDING / ACCOUNT_DISABLED / ACCOUNT_LOCKED / PASSWORD_CHANGE_REQUIRED / FORBIDDEN / CSRF_TOKEN_INVALID | 见上文 |
| 404 | NOT_FOUND | 资源不存在（不泄露存在性：统一文案） |
| 409 | CONFLICT | 用户名已存在 |
| 429 | RATE_LIMITED | 登录限流 |
| 500 | INTERNAL | 未捕获错误（响应不含内部细节） |

### 4.6 权限矩阵

| 接口 | user | sysadmin | secadmin | auditadmin |
|---|---|---|---|---|
| auth 四个接口 | 本人 | ✓ | ✓ | ✓ |
| `POST /users`、`PATCH password`、`POST disable` | – | ✓ | – | – |
| `POST approve/unlock`、sessions 枚举/强制下线 | – | – | ✓ | – |
| `GET /audit-logs` | – | – | – | ✓ |

约束：一个账号只持一个角色；同一自然人不得兼任两员（管理流程约束 + 审计可查，服务端不做自然人识别）。角色校验在**服务端逐路由**强制（中间件），与前端无关。

## 5. 核心流程

**双人开通**（等保三权分立）：
1. sysadmin `POST /users` → pending 账号，审计 `account.create`
2. secadmin `POST /users/:id/approve` → active，审计 `account.approve`
3. 用户登录（初始密码）→ 命中 `must_change_password` → 仅放行改密（新密码须通过历史校验，§6）→ 正常使用

**登录**：
1. 限流检查（IP）→ 锁定检查（用户名）→ 取用户 → argon2id verify
2. 失败 → 统一 401、计数、审计
3. 成功 → 状态检查 → 建会话 + csrf → 写审计 → 返回 user + csrf_token

**登出**：删 `sess:{sid}`、清 cookie、审计 `auth.logout`。

## 6. 密码策略与 argon2id 参数

- 参数：m=64MiB、t=3、p=4，salt 16B，hash 32B，PHC 标准编码（`alexedwards/argon2id`）
- 密码策略：最少 12 位；若大写/小写/数字/符号含至少两类则 12 位起，纯单类需 ≥16 位（口令短语友好）；不得与用户名相同或包含用户名；拒绝内置 top100 弱口令名单；**不得与当前密码及最近 5 次历史密码相同**
- 历史密码追踪：`password_history` 表保留每用户最近 5 条旧 hash。改密流程：校验新密码不与当前密码及 5 条历史匹配（逐条 argon2id verify，旧参数随各自 PHC 保存）→ 更新 users.password_hash → 将**旧**当前密码插入 history → 裁剪至 5 条（删除最旧）。sysadmin 重置密码同样走此流程
- 登录路径 ~100–300ms 的 argon2id 耗时是特性（抗离线爆破），不视为性能问题

## 7. 审计动作清单

`admin.bootstrap`、`auth.login_success`、`auth.login_failure`、`auth.logout`、`auth.password_change`、`auth.password_reset`、`account.create`、`account.approve`、`account.disable`、`account.unlock`、`session.force_logout`。每条含 actor（未登录则 username 为提交值、actor_id NULL）、target、ip、detail。

## 8. 代码结构

```
cmd/apiserver/
  main.go            # serve 子命令：迁移 + 起 gin server
  admin.go           # `apiserver admin bootstrap`：一次性创建三初始管理员（交互式输入各自密码，全部 must_change_password=true），仅在无管理员时可执行，幂等
internal/apiserver/
  handler/           # auth.go users.go sessions.go audit.go —— 薄层，只做绑定与状态码
  service/           # 业务逻辑（登录、锁定、双人状态机、踢会话），纯 Go 可单测
  middleware/        # requestid → recovery → securityheaders → accesslog → session → csrf → requireRole
  store/             # users.go audit.go（pgx）、session.go lock.go（go-redis）
pkg/store/migrations/ # golang-migrate SQL 文件（0001_users.up/down.sql、0002_audit_logs.up/down.sql）
```

安全响应头中间件（所有 API 响应）：`X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: no-referrer`、`Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`。HSTS 由网关终止 TLS 时添加，apiserver 不重复。

中间件行为要点：
- **session**：解析 cookie → Redis 查 `sess:{sid}` → Redis 错误或不存在 → 一律 401 `SESSION_EXPIRED`（fail-closed）→ 绝对过期判断（`created_at + 12h`）→ 通过则 touch TTL（滑动续期）→ `c.Set("user", user)`
- **csrf**：非 GET/HEAD/OPTIONS 且非 `/auth/login` 的会话请求，比对 header 与会话 `csrf` 字段
- **requireRole(roles...)**：从会话用户取角色，不在列表 → 403 `FORBIDDEN`
- **recovery**：panic → 500 统一响应 + 请求 ID；不泄露堆栈到响应体

## 9. 初始化与运维

- `apiserver admin bootstrap`：在 users 表无任何管理员（role in 三员）时可执行；创建 sysadmin/secadmin/auditadmin 三个账号（pending? 否——直接 active 但 `must_change_password=true`，否则无人可审首批）。幂等：已存在管理员则拒绝执行并提示。
- Redis 故障：fail-closed，所有受保护接口 401/403，登录不可用（可用性换安全，等保取向）。
- Postgres 迁移随进程启动执行（golang-migrate），失败即退出。

## 10. 测试策略

- **单测**（不依赖外部服务）：
  - argon2id PHC 编解码与 verify、密码策略校验器
  - session/csrf/requireRole 中间件（gin 测试上下文 + miniredis）
  - 锁定计数器逻辑、双人状态机非法跃迁（pending→disable 等）
  - 各 handler 参数校验与错误码映射（service 打桩）
- **集成测**（testcontainers：Postgres + Redis）：
  - 登录 → me → 改密 → 旧会话失效路径
  - 双人开通全链路、停用踢会话、重置密码踢会话
  - 5 次失败锁定 → secadmin 解锁
  - csrf 缺失/错误 403；角色越权 403
  - 审计写入与 auditadmin 只读（含验证 UPDATE/DELETE 被拒）
- **e2e**（后续 `test/e2e`，本期不实现）：swagger/契约导出后对 §4 全表跑一遍。

## 11. 开放问题（实现前需确认）

1. ~~历史密码追踪~~ **已定（m00384）：做**，见 §3.1 password_history 表与 §6
2. ~~图形验证码~~ **已定（m00384）：不做**，锁定 + 限流已覆盖等保登录失败处理要求
3. ~~sessions 分页默认值~~ **已定（m00384）**：page_size=50，created_at 倒序
