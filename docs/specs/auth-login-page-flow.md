# 登录页流程 Spec（v1 定稿）

日期：2026-11-04 · 状态：定稿 · 关联：wayfinder 票 #16（地图 #14）、#15 工程决策、#17 dev 调研

> API 契约唯一真源：docs/specs/2026-11-02-login-design.md §4（已合并 main，PR #13）。本文件取代 #5 草案——草案的页面结构（3 页）与 AntD 默认主题决议维持，但 API 细节（Bearer token、localStorage、`/auth/change-password`、423 锁定码）全部作废。
> 工程决策以 #15 决议为准：Vite + React 18 + TS + AntD v5 + react-router、vite dev proxy、vitest、中英双语 i18n。

## 页面清单（v1 共 3 页）

| 页面 | 路由 | 说明 |
|---|---|---|
| 登录页 | `/login` | 用户名 + 密码 |
| 修改密码页 | `/change-password` | 首登强制，改完才能进控制台 |
| 控制台外壳 | `/` | v1 占位：顶栏（显示用户名 + 语言切换 + 登出） |

## 1. 登录页 `/login`

```
+--------------------------------------------------+
|                                                  |
|                [ model-go 控制台 ]                |
|                                                  |
|   用户名  [__________________________]            |
|   密  码  [__________________________]            |
|                                                  |
|              [ 登  录 ]                           |
|                                                  |
|   （报错区，居中于按钮下方，红字一行）              |
+--------------------------------------------------+
```

- 字段：username（必填）、password（必填，输入框 type=password）。空字段本地提示，不打接口。
- 提交：`POST /api/v1/auth/login`，成功 → 200 `{user:{id,username,role,status,must_change_password}, csrf_token}` + `Set-Cookie: __Host-session`（HttpOnly，前端不接触凭证）。
  - `user.must_change_password=true` → 跳 `/change-password`。
  - 否则 → 跳 `/`。
- 报错映射（按响应 `code` 取 i18n 词条，不直显后端 `message`）：

| HTTP/code | 词条（中文，en 另行翻译） |
|---|---|
| 401 `INVALID_CREDENTIALS` | 用户名或密码错误（用户不存在/密码错/锁定中统一文案，不区分） |
| 403 `ACCOUNT_PENDING` | 账号待安全管理员审核启用 |
| 403 `ACCOUNT_DISABLED` | 账号已停用，请联系管理员 |
| 429 `RATE_LIMITED` | 操作过于频繁，请稍后再试 |

- 登录成功后：`csrf_token` 存内存（刷新页面后从 `GET /auth/me` 重新获取，不落盘）；会话 cookie 由浏览器管理。

## 2. 修改密码页 `/change-password`

```
+--------------------------------------------------+
|                首次登录，请修改密码                 |
|                                                  |
|   初始密码    [__________________________]        |
|   新密码      [__________________________]        |
|               至少 12 位；不得与用户名相同          |
|   确认新密码  [__________________________]        |
|                                                  |
|              [ 确认修改 ]                         |
+--------------------------------------------------+
```

- 字段：oldPassword（初始密码）、newPassword、confirm（前端比对两次输入一致才提交）。
- 密码策略提示文案（最终校验在后端，前端仅提示规则）：≥12 位（含大写/小写/数字/符号中至少两类 12 位起，纯单类需 ≥16 位）；不得与用户名相同或包含用户名；拒绝常见弱口令；不得与当前密码及最近 5 次历史密码相同。
- 提交：`POST /api/v1/auth/password`，请求体 `{old_password, new_password}` → 204。服务端同步完成：清 `must_change_password`、踢掉该用户其他会话（保留当前）。
- 成功 → 跳 `/`。
- 到达条件：仅 `user.must_change_password=true` 可到达；已改密用户访问本路由直接重定向到 `/`。
- 守卫：全局路由守卫基于 `GET /auth/me` 的 `user.must_change_password`——true 时一切非本页路由强制跳 `/change-password`（与后端 403 `PASSWORD_CHANGE_REQUIRED` 门禁一致，前端直达也过不去）。

## 3. 控制台外壳 `/`（v1 占位）

```
+---------------------------------------------------------------+
| model-go   训练任务  推理服务  集群        [中/EN] [user ▼]   |
+---------------------------------------------------------------+
|                                                               |
|          （v1 占位：登录成功，工作台待后续迭代）                |
|                                                               |
+---------------------------------------------------------------+
```

- 顶栏右侧：语言切换（中/EN，#15 i18n 决议）、用户菜单。
- 用户菜单 `退出登录` → `POST /api/v1/auth/logout`（带 `X-CSRF-Token` 头）→ 204（服务端删 Redis 会话、清 cookie）→ 清内存态 → 跳 `/login`。

## 全局约定（前端）

1. **凭证**：会话为 `__Host-session` cookie（HttpOnly / Secure / SameSite=Lax / Path=/）。前端代码不读写该 cookie，不发 `Authorization` 头。
2. **CSRF**：`csrf_token` 取自 login / `GET /auth/me` 响应体，存内存；除 `POST /auth/login` 外，所有非 GET 请求带 `X-CSRF-Token: <csrf_token>` 头。收到 403 `CSRF_TOKEN_INVALID`：重新拉 `/auth/me` 取新 token 重试一次，再失败跳 `/login`。
3. **会话恢复**：应用加载先 `GET /auth/me`；401 `SESSION_EXPIRED` → 跳 `/login`。任意接口 401 同处理；403 `PASSWORD_CHANGE_REQUIRED` → 跳 `/change-password`。
4. **路由守卫**：react-router 实现，数据源为 `/auth/me`（登录态 + `must_change_password`），与后端中间件逐路由强校验互为冗余。
5. **dev 联调**：vite dev server proxy \`/api\` → \`http://localhost:8080\`（同源，零后端改动）；Safari ≤26 用户走 mkcert 本地 https 回退（详见 docs/specs/research-console-cookie-session-dev.md）。
6. **i18n**：react-i18next + i18next-browser-languagedetector，浏览器语言探测、默认中文，顶栏切换；AntD 组件 locale 用 ConfigProvider 同步；所有报错/文案按错误码映射本地词条。
7. **主题**：AntD 默认主题，v1 无自定义视觉设计（#5 决议维持）。
8. **安全响应头**：由 apiserver 统一输出（含 `CSP: default-src 'none'`），console 静态资源如需独立部署须另行评估——v1 由 dev proxy 覆盖，不涉及。
