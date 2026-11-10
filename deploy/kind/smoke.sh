#!/usr/bin/env bash
# kind 联调环境 smoke：通过 ingress（默认 http://localhost）走通
# 登录 → must_change 标志 → 强制改密 → /auth/me → 登出 → 新密码重登。
# 用法：./smoke.sh [BASE_URL] [初始密码] [新密码]
set -euo pipefail
BASE="${1:-http://localhost}"
OLDPW="${2:-Hub-dev#2026-a}"
NEWPW="${3:-Hub-dev#2026-a-rotated}"
JAR="$(mktemp)"

step() { printf '\n== %s ==\n' "$1"; }
expect() { # expect <描述> <期望码> <实际码>
  if [ "$2" != "$3" ]; then echo "FAIL: $1 期望 HTTP $2 实得 $3"; exit 1; fi
  echo "OK: $1 -> $3"
}

step "console SPA 可达"
expect "GET /"            200 "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/")"
expect "GET /login (SPA fallback)" 200 "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/login")"

step "初始密码登录（响应体带 csrf_token + must_change 标志）"
BODY="$(curl -s -c "$JAR" -H 'Content-Type: application/json' -d "{\"username\":\"sysadmin\",\"password\":\"$OLDPW\"}" -w '\n%{http_code}' "$BASE/api/v1/auth/login")"
CODE="${BODY##*$'\n'}"; JSON="${BODY%$'\n'*}"
expect "POST /auth/login" 200 "$CODE"
CSRF="$(printf '%s' "$JSON" | sed -E 's/.*"csrf_token":"([^"]+)".*/\1/')"
echo "$JSON" | grep -q '"must_change_password":true' || { echo "FAIL: 首次登录应 must_change_password=true: $JSON"; exit 1; }
echo "OK: must_change_password=true, csrf 已取得"

step "强制改密"
BODY="$(curl -s -b "$JAR" -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" -d "{\"old_password\":\"$OLDPW\",\"new_password\":\"$NEWPW\"}" -w '\n%{http_code}' "$BASE/api/v1/auth/password")"
CODE="${BODY##*$'\n'}"
expect "POST /auth/password" 204 "$CODE"

step "改密后 /auth/me 放行"
BODY="$(curl -s -b "$JAR" -w '\n%{http_code}' "$BASE/api/v1/auth/me")"
CODE="${BODY##*$'\n'}"; JSON="${BODY%$'\n'*}"
expect "GET /auth/me" 200 "$CODE"
echo "$JSON" | grep -q '"must_change_password":false' || { echo "FAIL: 改密后标志应翻转: $JSON"; exit 1; }
echo "OK: must_change_password=false"

step "登出"
LO="$(curl -s -b "$JAR" -X POST -H "X-CSRF-Token: $CSRF" -o /dev/null -w '%{http_code}' "$BASE/api/v1/auth/logout")"
expect "POST /auth/logout" 204 "$LO"

step "新密码重登"
B3="$(curl -s -c "$JAR" -H 'Content-Type: application/json' -d "{\"username\":\"sysadmin\",\"password\":\"$NEWPW\"}" -o /dev/null -w '%{http_code}' "$BASE/api/v1/auth/login")"
expect "POST /auth/login (new pw)" 200 "$B3"

rm -f "$JAR"
printf '\nSMOKE-OK\n'
