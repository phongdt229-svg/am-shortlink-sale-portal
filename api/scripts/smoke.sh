#!/usr/bin/env bash
# Smoke test portal-api đang chạy (mặc định http://localhost:8080) trên dữ liệu cmd/seed.
set -u
B=${B:-http://localhost:8080}
j() { python -c "import sys,json;d=json.load(sys.stdin);print($1)"; }
login() { curl -s -X POST $B/v1/auth/login -H 'content-type: application/json' -d "{\"username\":\"$1\",\"password\":\"$2\"}"; }
code() { curl -s -o /dev/null -w "%{http_code}" "$@"; }

echo "healthz=$(code $B/healthz) readyz=$(code $B/readyz)"
echo "me không token: $(curl -s $B/v1/me)"
echo "login body sai: $(curl -s -X POST $B/v1/auth/login -H 'content-type: application/json' -d '{"username":""}')"
echo "sai mật khẩu: $(login partner_a x | j "d['status'],d['code']")"
echo "không portal_access: $(login partner_d partner123 | j "d['status'],d['code']")"
L=$(login viewer viewer123)
echo "login viewer: $(echo $L | j "d['user']")"
T=$(echo $L | j "d['access_token']"); R=$(echo $L | j "d['refresh_token']")
H="Authorization: Bearer $T"
echo "me: $(curl -s $B/v1/me -H "$H")"
echo "accounts: $(curl -s $B/v1/filters/accounts -H "$H")"
echo "campaigns partner_b: $(curl -s "$B/v1/filters/campaigns?account=partner_b" -H "$H")"
echo "campaigns partner_c (ngoài phạm vi): $(curl -s "$B/v1/filters/campaigns?account=partner_c" -H "$H" | j "d['status'],d['code']")"
echo "prefixes: $(curl -s $B/v1/filters/prefixes -H "$H")"
echo "limit=999: $(curl -s "$B/v1/filters/accounts?limit=999" -H "$H")"
R2=$(curl -s -X POST $B/v1/auth/refresh -H 'content-type: application/json' -d "{\"refresh_token\":\"$R\"}" | j "d['refresh_token']")
echo "refresh: ${R2:0:12}…"
echo "logout: $(code -X POST $B/v1/auth/logout -H 'content-type: application/json' -d "{\"refresh_token\":\"$R2\"}")"
echo "refresh sau logout: $(curl -s -X POST $B/v1/auth/refresh -H 'content-type: application/json' -d "{\"refresh_token\":\"$R2\"}" | j "d['status'],d['code']")"
A=$(login admin admin123 | j "d['access_token']")
echo "admin campaigns q=play: $(curl -s "$B/v1/filters/campaigns?q=play" -H "Authorization: Bearer $A")"
echo "404: $(curl -s $B/v1/nope -H "Authorization: Bearer $A")"
echo "metrics http_server series: $(curl -s $B/metrics | grep -c '^http_server')"
