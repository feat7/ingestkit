#!/bin/sh
# Generate local configuration only; never execute values or overwrite secrets.
set -eu
umask 077
fail() { printf '%s\n' "$1" >&2; exit 1; }
ingestkit_domain=${1-}
ingestkit_output=${2-}
[ "$#" -ge 1 ] && [ "$#" -le 2 ] || fail 'Usage: configure.sh HOSTNAME [OUTPUT_DIRECTORY]'
[ "${#ingestkit_domain}" -le 253 ] || fail 'Invalid hostname'
case "$ingestkit_domain" in ''|*[!A-Za-z0-9.-]*|.*|*.|*..*) fail 'Invalid hostname';; esac
case "$ingestkit_domain" in localhost|*.*) ;; *) fail 'Invalid hostname: use a fully qualified domain or localhost';; esac
ingestkit_labels=$(printf '%s' "$ingestkit_domain" | tr '.' ' ')
for ingestkit_label in $ingestkit_labels; do
    [ "${#ingestkit_label}" -le 63 ] || fail 'Invalid hostname label'
    case "$ingestkit_label" in -*|*-) fail 'Invalid hostname label';; esac
done
if [ -z "$ingestkit_output" ]; then
    ingestkit_output=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
fi
ingestkit_env=$ingestkit_output/.env
[ ! -e "$ingestkit_env" ] && [ ! -L "$ingestkit_env" ] || fail 'Configuration already exists; refusing to overwrite it'
command -v openssl >/dev/null 2>&1 || fail 'OpenSSL is required to generate secrets'
ingestkit_admin=$(openssl rand -hex 32) || fail 'Could not generate database secret'
ingestkit_writer=$(openssl rand -hex 32) || fail 'Could not generate writer secret'
ingestkit_key=$(openssl rand -hex 32) || fail 'Could not generate API secret'
# Validate generator output too: a broken command must not create weak config.
for ingestkit_secret in "$ingestkit_admin" "$ingestkit_writer" "$ingestkit_key"; do
    [ "${#ingestkit_secret}" -eq 64 ] || fail 'Secret generation returned an invalid value'
    case "$ingestkit_secret" in *[!0-9a-f]*) fail 'Secret generation returned an invalid value';; esac
done
mkdir -p -- "$ingestkit_output"
# noclobber makes creation exclusive, including a competing initializer.
(set -C; cat > "$ingestkit_env" <<CONFIG
DOMAIN=$ingestkit_domain
API_KEYS={"$ingestkit_key":"default"}
POSTGRES_ADMIN_PASSWORD=$ingestkit_admin
POSTGRES_WRITER_PASSWORD=$ingestkit_writer
CONFIG
) || fail 'Could not create configuration; no existing file was overwritten'
printf 'Created private configuration: %s\n' "$ingestkit_env"
