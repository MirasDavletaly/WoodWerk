# Развёртывание WOODWERK на сервере

Пошагово, от чистого Ubuntu/Debian до работающего сайта с HTTPS.
Всё, что нужно, уже лежит в папке `deploy/`.

---

## Что понадобится

- Сервер с Ubuntu 22.04+ или Debian 12+ и правами `sudo`
- Домен, у которого A-запись указывает на IP этого сервера
- 1 ГБ памяти и 2 ГБ диска — этого хватает с запасом

Go на сервере **не нужен**: собранные программы лежат в `server/build/`.

---

## 1. Загрузить файлы

Со своего компьютера, из папки проекта:

```bash
rsync -av --exclude '.git' --exclude 'data' --exclude 'uploads' \
      ./ user@ваш-сервер:/tmp/woodwerk/
```

Если `rsync` нет — заархивируйте папку и распакуйте на сервере:

```bash
scp woodwerk.zip user@ваш-сервер:/tmp/
ssh user@ваш-сервер 'cd /tmp && unzip woodwerk.zip -d woodwerk'
```

## 2. Установить

На сервере:

```bash
cd /tmp/woodwerk
sudo bash deploy/install.sh
```

Скрипт заведёт системного пользователя `woodwerk`, разложит файлы
в `/var/www/woodwerk`, поставит службу systemd и запустит её.
В конце он сам проверит, что сайт отвечает.

Повторный запуск скрипта обновляет сайт и **не трогает** базу,
фотографии и заявки.

## 3. Nginx и HTTPS

```bash
sudo apt install nginx certbot python3-certbot-nginx

sudo cp /var/www/woodwerk/deploy/nginx.conf /etc/nginx/sites-available/woodwerk
sudo nano /etc/nginx/sites-available/woodwerk     # заменить woodwerk.kz на свой домен
sudo ln -s /etc/nginx/sites-available/woodwerk /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d ваш-домен -d www.ваш-домен
```

Certbot сам пропишет сертификат и настроит автопродление.

### Почему важно ставить nginx именно так

В образце `deploy/nginx.conf` подключается `proxy_params` — он передаёт
серверу заголовки `X-Real-IP` и `X-Forwarded-Proto`. Без них:

- **лимит попыток входа станет общим на всех.** За прокси все посетители
  выглядят как `127.0.0.1`, и десяти неудачных попыток от постороннего
  хватит, чтобы вход закрылся и для вас;
- **кука сессии уйдёт без флага `Secure`**, потому что сам Go-сервер
  работает по http и о вашем сертификате не знает.

Этим заголовкам сервер верит только когда соединение пришло с петлевого
адреса — то есть от nginx на той же машине. Подделать их снаружи нельзя.

## 4. Первый вход

Откройте `https://ваш-домен/admin`

| | |
| --- | --- |
| Логин | `admin` |
| Пароль | `admin` |

> ### Смените пароль сразу
>
> Пока стоит `admin/admin`, войти может **любой, кто знает адрес сайта** —
> а боты перебирают такие пары в первые же часы после запуска домена.
> Через панель можно залить файлы и стереть весь каталог.
>
> Панель будет показывать красную полосу, пока пароль не изменён:
> **Настройки → Смена пароля**.
>
> Забыли свой пароль — вернуть стандартный:
> ```bash
> sudo systemctl stop woodwerk
> sudo -u woodwerk /var/www/woodwerk/server/build/woodwerk-linux-amd64 \
>      -dir /var/www/woodwerk -reset-admin -addr 127.0.0.1:9999 &
> sleep 3 && sudo pkill -f 'addr 127.0.0.1:9999'
> sudo systemctl start woodwerk
> ```

---

## Повседневное

| Что | Команда |
| --- | --- |
| Журнал в реальном времени | `journalctl -u woodwerk -f` |
| Последние 50 строк | `journalctl -u woodwerk -n 50` |
| Перезапуск | `sudo systemctl restart woodwerk` |
| Остановка | `sudo systemctl stop woodwerk` |
| Состояние | `systemctl status woodwerk` |

### Обновление сайта

Загрузите новые файлы и повторите `sudo bash deploy/install.sh` —
данные останутся на месте.

### Резервные копии

```bash
sudo bash /var/www/woodwerk/deploy/backup.sh
```

Складывает в `/var/backups/woodwerk`, хранит 30 последних копий.
Для ежедневной копии в 4 утра:

```bash
sudo crontab -e
# добавить строку:
0 4 * * * /bin/bash /var/www/woodwerk/deploy/backup.sh >/dev/null 2>&1
```

В копию попадают база (`data/woodwerk.db`), фотографии (`uploads/`)
и заявки (`leads.jsonl`) — всё, что нельзя восстановить из репозитория.

> База работает в режиме WAL, поэтому просто скопировать файл на живом
> сервере нельзя — получится испорченный снимок. Скрипт использует
> `sqlite3 .backup`, который снимает согласованную копию не останавливая сайт.
> Если `sqlite3` не установлен, он на пару секунд остановит службу.

### Восстановление из копии

```bash
sudo systemctl stop woodwerk
cd /var/www/woodwerk
sudo tar -xzf /var/backups/woodwerk/woodwerk_ДАТА.tar.gz -C /tmp/restore
sudo cp /tmp/restore/woodwerk.db data/
sudo rm -rf uploads && sudo cp -a /tmp/restore/uploads uploads
sudo chown -R woodwerk:woodwerk data uploads
sudo systemctl start woodwerk
```

---

## Настройки запуска

Меняются в `/etc/systemd/system/woodwerk.service`, после правки —
`sudo systemctl daemon-reload && sudo systemctl restart woodwerk`.

| Флаг | По умолчанию | Зачем |
| --- | --- | --- |
| `-addr` | `:8090` | адрес и порт |
| `-dir` | `.` | каталог сайта |
| `-db` | `data/woodwerk.db` | файл базы |
| `-uploads` | `uploads` | куда класть фотографии |
| `-max-upload` | `5` | предел размера фотографии, МБ |
| `-leads` | `leads.jsonl` | файл заявок |
| `-hsts` | выкл. | слать HSTS — **только когда HTTPS уже работает** |
| `-admin-user` | `admin` | логин администратора |
| `-admin-pass` | — | задать пароль; на существующей учётке меняет его |
| `-reset-admin` | выкл. | вернуть пароль к `admin` |

`-hsts` в готовом файле службы уже включён. Если сертификата ещё нет,
уберите этот флаг, иначе браузеры запомнят требование HTTPS и сайт
перестанет открываться по http.

---

## Если что-то не работает

**Служба не стартует.** `journalctl -u woodwerk -n 50` — там будет причина.
Чаще всего: занят порт или нет прав на `data/`.

**502 Bad Gateway.** Nginx не достучался до сервера. Проверьте, что служба
жива (`systemctl status woodwerk`) и что порт в `nginx.conf` совпадает
с портом в `woodwerk.service`.

**Фотографии не загружаются.** Скорее всего упирается в лимит nginx —
в конфиге стоит `client_max_body_size 20m`, проверьте что он применился.

**Сайт открывается, но каталог пустой.** Значит не отвечает `/api/products`.
Смотрите журнал сервера — обычно это права на файл базы.

---

## Что проверить перед публикацией

- [ ] Сменён пароль администратора
- [ ] Домен в `robots.txt` и `sitemap.xml`
- [ ] Почта и телефон в контактах
- [ ] БИН в `contacts.html` (сейчас там заглушка `000000000000`)
- [ ] Юридические тексты в `privacy.html` — они написаны под российское
      право и для казахстанской компании их должен переписать юрист
- [ ] Способы оплаты: на страницах стоят «МИР» и «СБП» — замените на то,
      чем реально принимаете оплату
- [ ] Цены в каталоге — сейчас демонстрационные
- [ ] Настроены резервные копии
