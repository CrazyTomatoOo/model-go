# 调研：gin 认证、Redis 会话与密码哈希的库选型

> 来源：GitHub issue #2（wayfinder:research），为登录功能（#1）选定技术件。
> 日期：2026-10-31。所有结论均来自文末列出的官方文档/官方仓库。

## 推荐组合（结论）

1. **密码哈希：Argon2id**，`golang.org/x/crypto/argon2.IDKey`
   - 参数起步值：`m=64 MiB (65536 KiB), t=3, p=4`（RFC 9106 §4 低内存档，也是 x/crypto 官方文档第二推荐）；OWASP 最低门槛为 `m=19 MiB, t=2, p=1`。生产部署时在目标机器上实测，把单次哈希耗时调到 ~0.5–1 秒再定档。
   - salt 用 `crypto/rand` 生成 16 字节；输出 32 字节 key。
   - 参数随哈希一起以 **PHC 字符串**存储（`$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>`），便于将来升级参数（登录时按当前档位 rehash）。
2. **Redis 会话：自研轻量 token→user 中间件**，客户端用官方 `github.com/redis/go-redis/v9`（当前 v9.23.0，活跃维护，Redis 7+，支持 Sentinel/Cluster、RESP3）
   - 与 issue #2 既定方向一致（“不引全套 session 框架”）。Redis 官方文档的 Go session-store 指南就是这套模式，可直接照抄再包成 gin 中间件。
   - 形态：cookie 只放 opaque session id（`HttpOnly` + `Secure` + `SameSite`），服务端存 `session:{id}` Redis hash，`HSET`/`HGETALL`/`EXPIRE`（滑动过期）/`DEL`（登出），id 用 `crypto/rand` 生成。
3. **gin 中间件形态：两个中间件叠加**
   - `SessionMiddleware`（全局注册）：读 cookie → 查 `session:{id}` → 命中则 `c.Set("user_id", …)` 并续 TTL。
   - `RequireAuth`（挂在需登录路由组上）：`c.Get("user_id")` 为空则 401，否则放行。
   - 参考实现：Redis 官方 Go session-store 指南（net/http 版，可包成 gin 中间件）、`gin-contrib/sessions` 的示例。

## 密码哈希：选项与依据

| 方案 | 维护状态 | 一句话取舍 |
|---|---|---|
| **Argon2id**（golang.org/x/crypto/argon2，`IDKey`） | Go 团队维护；2015 PHC 冠军、RFC 9106 标准化 | ✅ 当前首选，内存难攻击 + 抗侧信道，无输入长度上限 |
| scrypt（golang.org/x/crypto/scrypt） | Go 团队维护 | OWASP 第二选择（N=2^17, r=8, p=1）；只有 argon2 不可用时才考虑 |
| bcrypt（golang.org/x/crypto/bcrypt） | Go 团队维护 | 仅遗留系统；72 字节输入上限、work factor ≥10，新项目不选 |
| PBKDF2（x/crypto/pbkdf2） | Go 团队维护 | 仅 FIPS-140 合规需要；HMAC-SHA256 需 600,000 次迭代 |

- OWASP Password Storage Cheat Sheet：密码哈希首选 Argon2id（最低 `m=19 MiB, t=2, p=1`），不可用时 scrypt（N=2^17, r=8, p=1），bcrypt 仅限遗留（work factor ≥10、72 字节限制）。
- `x/crypto/argon2` 官方文档：不确定时用 Argon2id（`IDKey`）；RFC 9106 §4 推荐档位一 `m=2 GiB, t=1, p=4`，内存受限档位二 `m=64 MiB, t=3, p=4`。

## Redis 会话：选项与依据

| 方案 | 维护状态 | 一句话取舍 |
|---|---|---|
| **自研轻量 token→user**（go-redis/v9） | —（自己维护，逻辑约 100 行） | ✅ 依赖最小、贴合无状态 apiserver、Redis 官方文档背书；哨兵/集群/连接池完全可控 |
| gin-contrib/sessions v1.1.2 | 活跃（gin 官方配套，多后端） | Redis 后端仍走 `gomodule/redigo` + `boj/redistore`（旧库，自建连接池，Sentinel/Cluster 支持弱）；换 go-redis 的 PR #148 关闭未合并；其 Redis 实现无法复用现有 go-redis 客户端 |
| gorilla/sessions + rbcervilla/redisstore/v9 | gorilla/sessions 活跃（最新需 go 1.23+）；redisstore 维护可 | 接口是 net/http 风格、非 gin 原生；redisstore 用 go-redis，支持单机/Sentinel/Cluster |
| Calidity/gin-sessions | fork，小众 | gin-contrib/sessions 的 fork，Redis 后端换成 go-redis/v9，支持 Sentinel/Cluster；长期维护风险 |
| rueidis（客户端层） | 活跃（v1.0.79，Apache-2.0，redis 官方生态） | 性能更优、带客户端缓存，但无现成 session store 集成，仍需自写；与 go-redis/v9 二选一 |

- Redis 官方 “Redis session store with Go” 指南：推荐模式即 opaque sid cookie + `session:{id}` hash + TTL，并明确说真实应用里“把同一套 Redis 操作包成 Gin/Echo/Fiber 的中间件”。这是自研方案的一手依据。
- go-redis/v9 是 Redis 官方 Go 客户端（README 自述 official Redis client library for Go），go.mod 最低 go 1.26（CI 亦跑旧 Go），与项目 go 1.24 适配时按需选对应版本。

## 备选评估小结（不采纳的原因）

- **gin-contrib/sessions**：整体活跃，但 Redis 后端是 redigo/redistore 老栈，正是 issue #2 想避开“全套框架”的另一面——不引它就不必为旧连接池买单。cookie/memstore 等简单场景可用。
- **gorilla/sessions 系列**：可靠但 net/http 风格，接入 gin 要多一层适配，且 cookie 加密密钥管理（keyPairs）对无状态 apiserver 不如直接 token→user 直观。
- **JWT 方案**（不在本次 ticket 范围）：无 Redis 依赖但无法服务端主动失效/登出，与 #1 登录 spec 的会话语义不符，故未展开。

## 来源链接

- OWASP Password Storage Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
- golang.org/x/crypto/argon2（官方文档）: https://pkg.go.dev/golang.org/x/crypto/argon2
- RFC 9106 Argon2（参数档位 §4）: https://rfc-editor.org/rfc/rfc9106.html
- Redis 官方 Go session store 指南: https://redis.io/docs/latest/develop/use-cases/session-store/go/
- go-redis（Redis 官方 Go 客户端）: https://github.com/redis/go-redis
- rueidis: https://github.com/redis/rueidis
- gin-contrib/sessions（v1.1.2）: https://github.com/gin-contrib/sessions
- gin-contrib/sessions PR #148（Redis 换库，关闭未合并）: https://github.com/gin-contrib/sessions/pull/148
- redigo: https://github.com/gomodule/redigo
- gorilla/sessions: https://github.com/gorilla/sessions
- rbcervilla/redisstore（gorilla 的 go-redis Redis store）: https://github.com/rbcervilla/redisstore
- Calidity/gin-sessions（go-redis/v9 fork）: https://github.com/Calidity/gin-sessions
