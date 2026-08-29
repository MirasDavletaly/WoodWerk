#!/usr/bin/env bash
#
# Установка WOODWERK на сервер с systemd.
#
# Запускать на сервере из распакованной папки сайта:
#     sudo bash deploy/install.sh
#
# Скрипт заводит пользователя, раскладывает файлы, ставит службу и
# запускает её. Nginx и сертификат — отдельно, см. DEPLOY.md.

set -euo pipefail

TARGET=/var/www/woodwerk
SERVICE=woodwerk
USER_NAME=woodwerk
BINARY=server/build/woodwerk-linux-amd64

say()  { printf '\n\033[1m%s\033[0m\n' "$*"; }
fail() { printf '\n\033[31mОшибка: %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "запустите через sudo"

SOURCE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SOURCE"

[ -f index.html ] || fail "запускать нужно из папки сайта (нет index.html)"

# Под какую архитектуру собран сервер
case "$(uname -m)" in
    x86_64)  BINARY=server/build/woodwerk-linux-amd64 ;;
    aarch64|arm64) BINARY=server/build/woodwerk-linux-arm64 ;;
    *) fail "неизвестная архитектура $(uname -m); соберите бинарник сами" ;;
esac
[ -f "$BINARY" ] || fail "не найден $BINARY — соберите его командой go build"

say "1. Пользователь $USER_NAME"
if id "$USER_NAME" >/dev/null 2>&1; then
    echo "   уже есть"
else
    useradd --system --no-create-home --shell /usr/sbin/nologin "$USER_NAME"
    echo "   создан"
fi

say "2. Файлы сайта в $TARGET"
mkdir -p "$TARGET"
# Данные не трогаем: при повторной установке база и фотографии остаются.
rsync -a --delete \
    --exclude 'data/' \
    --exclude 'uploads/' \
    --exclude 'leads.jsonl' \
    --exclude '.git/' \
    --exclude 'tools/' \
    "$SOURCE"/ "$TARGET"/
mkdir -p "$TARGET/data" "$TARGET/uploads"
touch "$TARGET/leads.jsonl"
echo "   скопировано"

say "3. Права"
chown -R "$USER_NAME:$USER_NAME" "$TARGET"
chmod +x "$TARGET/$BINARY"
# Писать сервису можно только в данные, остальное — только чтение.
find "$TARGET" -type d -exec chmod 755 {} +
find "$TARGET" -type f -exec chmod 644 {} +
chmod +x "$TARGET/$BINARY"
chmod 700 "$TARGET/data" "$TARGET/uploads"
chmod 600 "$TARGET/leads.jsonl"
echo "   выставлены"

say "4. Служба systemd"
UNIT=/etc/systemd/system/$SERVICE.service
sed "s|woodwerk-linux-amd64|$(basename "$BINARY")|" \
    "$TARGET/deploy/woodwerk.service" > "$UNIT"
systemctl daemon-reload
systemctl enable "$SERVICE" >/dev/null
systemctl restart "$SERVICE"
sleep 2
echo "   установлена"

say "5. Проверка"
if systemctl is-active --quiet "$SERVICE"; then
    echo "   служба работает"
else
    systemctl status "$SERVICE" --no-pager -l | tail -20
    fail "служба не запустилась"
fi

if curl -fsS -o /dev/null http://127.0.0.1:8090/api/categories; then
    echo "   сайт отвечает на 127.0.0.1:8090"
else
    fail "сайт не отвечает; смотрите journalctl -u $SERVICE -n 50"
fi

say "Готово"
cat <<'NOTE'
   Дальше:
   1. Настройте nginx — образец в deploy/nginx.conf
   2. Получите сертификат:  sudo certbot --nginx -d ваш-домен
   3. Откройте /admin, войдите под admin / admin
      и СРАЗУ смените пароль в разделе «Настройки»

   Журнал:      journalctl -u woodwerk -f
   Перезапуск:  systemctl restart woodwerk
   Бэкап:       /var/www/woodwerk/data/woodwerk.db и uploads/
NOTE
