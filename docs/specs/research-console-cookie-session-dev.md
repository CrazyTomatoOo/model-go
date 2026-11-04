# 调研：dev 环境 cookie 会话与 vite proxy 兼容性

> 来源：GitHub issue #17（wayfinder:research），为登录功能的本地开发链路（Vite dev server → Go apiserver）确认 `__Host-session` cookie 与 Vite proxy 的兼容性。
> 日期：2026-11-03。所有结论均来自文末列出的官方文档、浏览器/引擎源码与规范讨论，未采用二手转述。

## 结论（结论先行）

1. **`__Host-session`（Secure、SameSite=Lax、Path=/、无 Domain）在纯 http://localhost 上能否被浏览器接受，按浏览器分三档**：
   - **Firefox：可以**（Firefox 75+，2020-04；bug 1618113 起 Secure 对 localhost 放行；httpwg #2605 实测前缀同样可用）。
   - **Chrome（现行版本）：可以**（现行源码把 localhost 与回环地址判为可信来源，前缀校验与 Secure 校验共用该判定；2023 年 httpwg 实测矩阵显示旧版 Chrome 拒绝前缀，用旧版本需实测或升级）。
   - **Safari：26.x 及更早全部拒绝；27.0+ 放行 Secure（loopback）**（WebKit PR #61338，2026-04-03 合并；Safari 27.0 release notes Networking 章节明确；前缀在 27.0 上无单独文档，需实测）。
2. **Vite proxy 对 Set-Cookie 零改动**：Vite 7.1.0+ 的 dev proxy 基于 http-proxy-3（http-party `http-proxy` 的兼容重写，选项语义一致）；http-proxy 默认对响应头（含 Set-Cookie）不做任何改写；`changeOrigin` 只改 Host 头；`cookieDomainRewrite`/`cookiePathRewrite` 是可选显式改写，不开即透传。Go 端 Set-Cookie 无 Domain（host-only）→ 浏览器把它原样存为 localhost:5173 的 host-only cookie，后续请求自动回传。
3. **推荐代理配置**：见 §3 的 `vite.config.ts` 片段（`/api` → `http://localhost:8080`，`changeOrigin: true` 即可，无需任何 cookie 改写项）。
4. **Safari < 27（及历史 Chrome）会拒 cookie** → 走 mkcert 本地 https 最稳（dev/prod cookie 形态零分叉）；dev 模式"去掉 Secure"单独做不够（`__Host-` 前缀校验要求 Secure 属性 + 安全来源，去 Secure 必须同时改名），详见 §4。

## 1. `__Host-` + `Secure` 在 http://localhost 的行为（Q1）

### 1.1 规则背景

- **MDN Set-Cookie**（2026-09 版）：
  - Secure："Indicates that the cookie is sent to the server only when a request is made with the `https:` scheme (except on localhost)"，且 "The `https:` requirements are ignored when the `Secure` attribute is set by localhost" —— 规范文档层面已承认 localhost 例外。
  - Cookie 前缀：`__Host-` 必须满足 Secure、无 Domain、Path=/（本项目 cookie 恰好满足；无 Domain → host-only）。
- **rfc6265bis（cookie 规范草案）**：PR #2759 "Support potentially trustworthy origins" 把 Secure 的判定从"https scheme"重写为"用户代理定义的安全连接（secure connection）"，httpwg issue #2605（浏览器不一致矩阵）随之关闭 —— 各浏览器分歧的根源正是对"localhost 是否算安全来源"的实现差异。

### 1.2 分浏览器结论与依据

| 浏览器 | Secure cookie（http://localhost） | `__Host-`/`__Secure-` 前缀 | 版本边界与依据 |
|---|---|---|---|
| Firefox | ✅ | ✅ | bugzilla 1618113 "Allow 'secure' cookies when set by localhost" RESOLVED FIXED，目标里程碑 mozilla75 → **Firefox 75（2020-04-07）**；httpwg #2605（2023 实测：Firefox 三类全部通过） |
| Chrome（现行） | ✅ | ✅ | chromium 源码（main，2026-11）：cookie_util.cc:701-706 `ProvisionalAccessScheme` 把 http 的 localhost 判为 `kTrustworthy`；cookie_util.cc:102-113 `HasValidSecurePrefixAttributes` 复用该判定（`!secure → false`，localhost → 通过）；cookie_base.cc:118-146 回传侧同样放行；net/base/url_util.cc:469-478 `IsLocalhost` = "localhost" 主机名 + 回环 IP（127.0.0.0/8、::1）。consentmodehq 2026-04 实测：Chrome 152 在 127.0.0.1 与 localhost 上均保留 Secure cookie（未测前缀） |
| Chrome（历史） | ✅ | ❌ | httpwg #2605（2023 实测：Chrome 收 Secure、拒前缀）；issues.chromium.org/40202941 以此为题的追踪项（本环境无法访问该追踪器，状态未确认）—— 旧版 Chrome 需实测，或直接走 https（§4） |
| Safari | ❌（≤26.x）→ ✅（27.0+） | 跟随 Secure 判定（27.0+ 无独立文档，需实测） | WebKit PR #61338（2026-04-03 合并，"Add support for `Secure` cookies for loopback hostnames"，merge aa297f89）：CookieJar::shouldIncludeSecureCookies 由 `url.protocolIs("https")` 改为 `shouldTreatAsPotentiallyTrustworthy(url)`（`HAVE(LOCALHOST_TIED_TO_LOOPBACK)` 门控）；Safari 27.0 release notes（Networking 原文："For loopback hosts using plaintext HTTP, cookies marked Secure can now be set via JavaScript and Set-Cookie headers"）；httpwg #2605（旧 Safari 全拒） |

### 1.3 对本项目的影响

- 目标场景（浏览器 → http://localhost:5173，纯 http）：**Firefox 与现行 Chrome 开箱即用；Safari 26.x 及更早会静默丢弃 Set-Cookie**（DevTools Network 中可见该响应头被浏览器拒收）。Safari 用户要么升 27.0+，要么走 §4 的 https。
- 127.0.0.1 与 localhost 等价：Chrome `IsLocalhost` 覆盖回环 IP；WebKit 改动明确 loopback；Firefox 走 potentially-trustworthy 定义。
- cookie 不绑定端口：host-only cookie 只按 host 归属，localhost:5173 与 localhost:8080 之间无端口区分问题。

## 2. Vite proxy 对 Set-Cookie 的处理（Q2）

- **Vite 文档**（server.proxy）：proxy 规则为 `{ key: ProxyOptions }`；`ProxyOptions` 继承 http-proxy-3 的选项，另加 `rewrite`（路径重写）、`configure`（拿到 proxy 实例）、`bypass`；示例即 `'/api': { target: 'http://localhost:8080', changeOrigin: true }`。
- **Vite 用哪个库**：Vite 7.1.0 起由 http-party `http-proxy` 换成 `http-proxy-3`（PR #20402，milestone 7.1，2025-07 合并；现行 8.x 用 http-proxy-3 ^1.23.x）；http-proxy-3 是 http-proxy 的兼容重写（README 自述沿用原指南并更新），选项名与语义一致。
- **http-proxy 的 cookie 相关选项**（http-party README 原文）：
  - `changeOrigin: true/false, Default: false - changes the origin of the host header to the target URL` —— **只改 Host 头**，与 cookie 无关。
  - `cookieDomainRewrite`：改写 Set-Cookie 的 Domain。默认不做；`false` = 不改；字符串 = 替换为新域名；`""` = 去掉 Domain；对象 = 按域名映射（`"*"` 通配）。
  - `cookiePathRewrite`：同上结构改写 Set-Cookie 的 Path。
- **Vite 中间件源码**（packages/vite/src/node/server/middlewares/proxy.ts）：全文件无任何 Set-Cookie/cookie 处理，只有 `rewrite`/`bypass`/`rewriteWsOrigin`（WebSocket 的 Origin 头）；target 以字符串给出时默认 `changeOrigin: true`。
- **结论**：`Set-Cookie: __Host-session=…; Secure; HttpOnly; SameSite=Lax; Path=/`（无 Domain）经 Vite proxy **原样**送达浏览器；浏览器按响应来源（localhost:5173）存为 host-only cookie，后续同源请求自动携带，Vite 再转发到 apiserver。cookie 能否存活只取决于 §1 的浏览器判定，proxy 不构成额外障碍，`Secure` 属性也不会被剥掉。

## 3. 推荐的 vite.config 片段（Q3）

```ts
// vite.config.ts
import { defineConfig } from 'vite'

export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true, // 只把 Host 头改成 localhost:8080；Set-Cookie 原样透传
        // 不需要 cookieDomainRewrite/cookiePathRewrite：
        // apiserver 的 Set-Cookie 无 Domain、Path=/，host-only cookie 直接落到 localhost:5173
      },
    },
  },
})
```

注意：
- 前端 fetch 与页面同源（localhost:5173），默认携带 cookie（SameSite=Lax 下同站请求不受限）；`X-CSRF-Token` 走响应体 JSON，proxy 不触碰。
- 约束：**apiserver 的 Set-Cookie 必须保持"无 Domain"**。一旦设了 Domain（哪怕 Domain=localhost），http-proxy 默认不改写，浏览器按域 cookie 规则处理（与 host-only 有微妙差异）；若日后 apiserver 换成非 localhost 主机名，才需要 `cookieDomainRewrite`（且 SameSite=Lax 的跨站语义会介入）。

## 4. 回退方案（Q4，按推荐顺序）

1. **mkcert 本地 https（推荐）**：`brew install mkcert && mkcert -install` 安装本地 CA，`mkcert localhost 127.0.0.1` 出证书，配进 Vite `server.https`。所有浏览器（含 Safari <27、旧版 Chrome）下 Secure 与 `__Host-` 前缀照常工作；dev/prod cookie 形态完全一致，零代码分叉。成本：每台机器装一次 CA；Safari 需在钥匙串里对该 CA 设为"始终信任"。
2. **dev 模式去掉 Secure**：**单独去 Secure 不够** —— Chrome 前缀校验 `HasValidSecurePrefixAttributes`（cookie_util.cc:102-113）要求"Secure 属性存在 **且** 来源可信"，去 Secure 后 `__Host-` 前缀直接非法，必须把 cookie 名改成 `session`（去前缀）。由此 dev/prod 的 cookie 名与属性分叉，有把 dev 形态带上线的风险；只适合本地冒烟，且 apiserver 必须用环境变量显式开关、默认仍走 Secure。
3. **不采纳**：仅把 localhost 换成 127.0.0.1（行为等价，无收益）；Vite 上配自签但未受信任的证书（浏览器仍拒）。

## 来源链接

- MDN Set-Cookie（Secure 与前缀规则）: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
- Vite server.proxy 文档: https://vite.dev/config/server-options（#server-proxy）
- Vite 改用 http-proxy-3 的 PR #20402: https://github.com/vitejs/vite/pull/20402
- Vite proxy 中间件源码（proxy.ts）: https://github.com/vitejs/vite/blob/main/packages/vite/src/node/server/middlewares/proxy.ts
- http-proxy（http-party）README: https://github.com/http-party/node-http-proxy
- http-proxy-3 README: https://github.com/sagemathinc/http-proxy-3
- chromium net/cookies 校验源码（main，2026-11）：cookie_util.cc / cookie_base.cc / canonical_cookie.cc / net/base/url_util.cc
- httpwg issue #2605（浏览器行为不一致矩阵）: https://github.com/httpwg/http-extensions/issues/2605
- rfc6265bis PR #2759（potentially trustworthy origins）: https://github.com/httpwg/http-extensions/pull/2759
- Firefox bug 1618113: https://bugzilla.mozilla.org/show_bug.cgi?id=1618113
- WebKit PR #61338: https://github.com/WebKit/WebKit/pull/61338
- Safari 27.0 release notes: https://webkit.org/blog/18325/webkit-features-for-safari-27-0/
- consentmodehq 2026-04 浏览器实测（Chrome 152 等）: https://consentmodehq.com/chrome-cookies-127-0-0-1-localhost/
- issues.chromium.org/40202941（Chrome http://localhost 上前缀被拒的追踪项；本环境无法访问，状态未确认）: https://issues.chromium.org/40202941
