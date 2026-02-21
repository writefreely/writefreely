#!/bin/sh
set -e

# 如果 config.ini 不存在，则根据环境变量生成
if [ ! -f /app/config.ini ]; then
cat > /app/config.ini << EOF
[server]
hidden_host      =
port             = ${WRITEFREELY_BIND_PORT:-8080}
bind             = ${WRITEFREELY_BIND_HOST:-0.0.0.0}
tls_cert_path    =
tls_key_path     =
autocert         = false
templates_parent_dir = /app
static_parent_dir    = /app
pages_parent_dir     = /app
keys_parent_dir      = /app
[database]
type     = sqlite3
filename = /app/writefreely.db
username =
password =
database =
host     =
port     = 3306
tls      = false
[app]
site_name         = ${WRITEFREELY_SITE_NAME:-My WriteFreely Blog}
site_description  =
host              = ${WRITEFREELY_HOST:-http://localhost:8080}
theme             = write
editor            =
disable_js        = false
webfonts          = true
landing           =
simple_nav        = false
wf_modesty        = false
chorus            = false
disable_drafts    = false
single_user       = ${WRITEFREELY_SINGLE_USER:-false}
open_registration = ${WRITEFREELY_OPEN_REGISTRATION:-true}
min_username_len  = ${WRITEFREELY_MIN_USERNAME_LEN:-2}
max_blogs         = ${WRITEFREELY_MAX_BLOG:-1}
federation        = ${WRITEFREELY_FEDERATION:-true}
public_stats      = ${WRITEFREELY_PUBLIC_STATS:-true}
private           = ${WRITEFREELY_PRIVATE:-false}
local_timeline    = ${WRITEFREELY_LOCAL_TIMELINE:-false}
user_invites      = ${WRITEFREELY_USER_INVITES:-}
default_visibility =
EOF
fi

# 初始化数据库（首次运行）
if [ ! -f /app/writefreely.db ]; then
  /app/writefreely -c /app/config.ini --init-db
  # 创建管理员账号
  /app/writefreely -c /app/config.ini --create-admin "${WRITEFREELY_ADMIN_USER:-admin}:${WRITEFREELY_ADMIN_PASS:-changeme}"
fi

exec /app/writefreely -c /app/config.ini
