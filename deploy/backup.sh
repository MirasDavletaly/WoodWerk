#!/usr/bin/env bash
#
# Резервная копия WOODWERK.
#
#     sudo bash /var/www/woodwerk/deploy/backup.sh [куда_класть]
#
# По умолчанию складывает в /var/backups/woodwerk и хранит 30 последних.
# Для ежедневной копии добавьте в crontab:
#     0 4 * * * /bin/bash /var/www/woodwerk/deploy/backup.sh >/dev/null 2>&1

set -euo pipefail

SITE=/var/www/woodwerk
DEST="${1:-/var/backups/woodwerk}"
KEEP=30
STAMP="$(date +%Y-%m-%d_%H%M)"

[ -d "$SITE" ] || { echo "нет каталога $SITE" >&2; exit 1; }
mkdir -p "$DEST"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# База в режиме WAL: копировать файл на живой системе нельзя, нужен
# .backup — он делает согласованный снимок, не останавливая сервер.
if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$SITE/data/woodwerk.db" ".backup '$WORK/woodwerk.db'"
else
    # sqlite3 не установлен — останавливаем службу на пару секунд.
    echo "sqlite3 не найден, останавливаю службу на время копии"
    systemctl stop woodwerk
    cp "$SITE/data/woodwerk.db" "$WORK/woodwerk.db"
    systemctl start woodwerk
fi

cp -a "$SITE/uploads" "$WORK/uploads"
[ -f "$SITE/leads.jsonl" ] && cp "$SITE/leads.jsonl" "$WORK/leads.jsonl"

ARCHIVE="$DEST/woodwerk_$STAMP.tar.gz"
tar -czf "$ARCHIVE" -C "$WORK" .
chmod 600 "$ARCHIVE"

echo "копия: $ARCHIVE ($(du -h "$ARCHIVE" | cut -f1))"

# Чистим старое, оставляя последние KEEP штук.
ls -1t "$DEST"/woodwerk_*.tar.gz 2>/dev/null | tail -n +$((KEEP + 1)) | while read -r old; do
    rm -f "$old"
    echo "удалена старая копия: $(basename "$old")"
done
