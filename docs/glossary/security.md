# Security / Auth Glossary

Terms resolved by the login/auth ticket (spec: `docs/specs/2026-11-02-login-design.md`). Implementation lives under `internal/apiserver`.

## Sessions

- **会话 (session)** — a server-side session identified by the opaque `__Host-session` cookie value. Redis key `sess:{sid}`, 256-bit base64url sid (43 chars). Sliding TTL 30 min per request (`SessionTTL`), absolute lifetime 12 h enforced by the session middleware (`SessionAbsLifetime`).
- **CSRF token** — 256-bit per-session token stored in Redis alongside the session, returned by `POST /api/v1/auth/login` and `GET /api/v1/auth/me`, sent back in the `X-CSRF-Token` header on state-changing requests. GET requests are exempt. Invalid/missing token → `403 CSRF_TOKEN_INVALID`.
- **Session middleware** — authenticates via the `__Host-session` cookie, re-resolves the user from Postgres every request, touches the sliding TTL, aborts with `401 SESSION_EXPIRED` when the session is absent/expired/absolute-lifetime passed or the user is gone.

## Login hardening

- **锁定 (lockout)** — Redis key `lock:{username}` counts consecutive login failures, refreshed TTL on every failure; 5 failures (`LockThreshold`) lock the account for 15 min (`LockTTL`). Locked attempts still return the unified 401.
- **限流 (rate limit)** — Redis key `rl:{ip}`; max 20 login attempts per IP per minute (`RateLimitPerMinute`, `RateLimitWindow`). The 21st → `429 RATE_LIMITED`.
- **统一 401** — every failed login (unknown user, wrong password, locked, inactive/pending) returns the same `401 INVALID_CREDENTIALS` body so attackers cannot enumerate accounts.
- **Pending 账号** — users default to `status=pending` and must be activated by a security admin before they can log in; pending login → `403 ACCOUNT_PENDING` (deliberately distinguishable, per spec).

## Bootstrap & roles

- **bootstrap** — `apiserver admin bootstrap` interactively creates the three initial admins `sysadmin`, `secadmin` (security admin), `auditadmin`, all `must_change_password=true`. Idempotent: refuses with exit 0 when admins already exist (`ErrAdminsExist`).
- **RBAC** — `users.role` in `sysadmin` / `secadmin` / `auditadmin` / `user`; `RequireRole` middleware gates routes, denying with `403 FORBIDDEN`.

## Audit trail

- **审计 (audit trail)** — `audit_logs` rows for `admin.bootstrap`, `auth.login_success`, `auth.login_failure` (reason in `detail` JSON: `invalid_password` / `user_not_found` / `locked` / `inactive`), `auth.logout`. Never return `password_hash` on any endpoint.
