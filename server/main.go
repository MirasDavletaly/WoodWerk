// Сервер сайта WOODWERK.
//
// Делает то, чего не умеет статический хостинг:
//  1. отдаёт заголовки безопасности (HSTS, X-Frame-Options, CSP и прочее);
//  2. принимает заявки с форм на POST /api/lead и складывает их в JSONL;
//  3. хранит каталог панелей в SQLite и отдаёт его сайту через /api/…;
//  4. поднимает админ-панель на /admin с авторизацией и загрузкой фотографий.
//
// Запуск:
//
//	go run ./server -addr :8090 -dir .
//
// При первом запуске создаётся учётная запись администратора; логин и пароль
// печатаются в журнал. Задать их заранее можно через ADMIN_USER и
// ADMIN_PASSWORD или флаги -admin-user и -admin-pass.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	// 8080 на рабочих машинах почти всегда занят Apache из XAMPP, OpenServer
	// или PostgreSQL, поэтому по умолчанию берём порт, который свободен чаще.
	addr := flag.String("addr", ":8090", "адрес и порт, например :8090")
	dir := flag.String("dir", ".", "каталог с файлами сайта")
	leadsPath := flag.String("leads", "leads.jsonl", "файл, куда дописываются заявки")
	dbPath := flag.String("db", "data/woodwerk.db", "файл базы данных SQLite")
	uploadsDir := flag.String("uploads", "uploads", "каталог для загруженных фотографий")
	maxUpload := flag.Int("max-upload", 5, "предельный размер одной фотографии, МБ")
	hsts := flag.Bool("hsts", false, "слать Strict-Transport-Security (только когда сайт реально за HTTPS)")
	adminUser := flag.String("admin-user", "", "логин администратора (по умолчанию admin)")
	adminPass := flag.String("admin-pass", "", "пароль администратора; на уже созданной учётной записи меняет пароль")
	resetAdmin := flag.Bool("reset-admin", false, "вернуть пароль администратора к admin (если забыли свой)")
	flag.Parse()

	root, err := resolveSiteDir(*dir)
	if err != nil {
		fatal("%v", err)
	}

	// Дальше все относительные пути считаем от каталога сайта, а не от того,
	// откуда программу запустили: при двойном щелчке это разные места.
	if err := os.Chdir(root); err != nil {
		fatal("не перейти в каталог сайта: %v", err)
	}

	// ---------------------------------------------------------------- база

	db, err := openDB(*dbPath)
	if err != nil {
		fatal("не открыть базу данных: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Seed(); err != nil {
		fatal("не заполнить базу: %v", err)
	}
	usingDefault, err := ensureAdmin(store, *adminUser, *adminPass, *resetAdmin)
	if err != nil {
		fatal("не создать администратора: %v", err)
	}
	if err := store.CleanExpiredSessions(); err != nil {
		logError(err)
	}

	uploads, err := NewUploads(*uploadsDir, *maxUpload)
	if err != nil {
		fatal("не создать каталог загрузок: %v", err)
	}

	leads, err := newLeadStore(*leadsPath)
	if err != nil {
		fatal("не открыть файл заявок: %v", err)
	}
	defer leads.Close()

	// ---------------------------------------------------------------- маршруты

	auth := NewAuth(store, *hsts)
	api := &API{store: store, uploads: uploads, auth: auth, logins: newLimiter()}
	api.setDefaultPassword(usingDefault)
	site := NewSite(root, auth)

	mux := http.NewServeMux()

	// Заявки с форм — как и раньше.
	mux.Handle("/api/lead", &leadHandler{store: leads, limiter: newLimiter()})

	// Публичное API: только активные изделия, только чтение.
	mux.HandleFunc("GET /api/products", api.publicProducts)
	mux.HandleFunc("GET /api/products/{id}", api.publicProduct)
	mux.HandleFunc("GET /api/categories", api.publicCategories)
	mux.HandleFunc("GET /api/gallery", api.publicGallery)
	mux.HandleFunc("GET /api/settings", api.publicSettings)

	// Вход и выход из админки.
	mux.HandleFunc("POST /api/admin/login", api.login)
	mux.HandleFunc("POST /api/admin/logout", api.logout)
	mux.HandleFunc("GET /api/admin/session", api.session)

	// Всё остальное в админке — только с действующей сессией.
	admin := http.NewServeMux()
	admin.HandleFunc("GET /api/admin/stats", api.stats)
	admin.HandleFunc("GET /api/admin/products", api.adminProducts)
	admin.HandleFunc("POST /api/admin/products", api.createProduct)
	admin.HandleFunc("GET /api/admin/products/{id}", api.adminProduct)
	admin.HandleFunc("PUT /api/admin/products/{id}", api.updateProduct)
	admin.HandleFunc("DELETE /api/admin/products/{id}", api.deleteProduct)
	admin.HandleFunc("PATCH /api/admin/products/{id}/status", api.setProductStatus)
	admin.HandleFunc("GET /api/admin/categories", api.adminCategories)
	admin.HandleFunc("POST /api/admin/categories", api.createCategory)
	admin.HandleFunc("PUT /api/admin/categories/{id}", api.updateCategory)
	admin.HandleFunc("DELETE /api/admin/categories/{id}", api.deleteCategory)
	admin.HandleFunc("GET /api/admin/gallery", api.adminGallery)
	admin.HandleFunc("POST /api/admin/gallery", api.createGalleryItem)
	admin.HandleFunc("PUT /api/admin/gallery/{id}", api.updateGalleryItem)
	admin.HandleFunc("DELETE /api/admin/gallery/{id}", api.deleteGalleryItem)
	admin.HandleFunc("POST /api/admin/gallery/reorder", api.reorderGallery)
	admin.HandleFunc("GET /api/admin/settings-site", api.adminSiteSettings)
	admin.HandleFunc("PUT /api/admin/settings-site", api.updateSiteSettings)
	admin.HandleFunc("POST /api/admin/upload", api.upload)
	admin.HandleFunc("POST /api/admin/upload/delete", api.deleteUpload)
	admin.HandleFunc("POST /api/admin/password", api.changePassword)
	admin.HandleFunc("POST /api/admin/username", api.changeUsername)
	mux.Handle("/api/admin/", auth.Protect(admin))

	// Загруженные фотографии.
	mux.Handle("GET /uploads/", uploads.Handler())

	// Страницы админ-панели.
	mux.HandleFunc("GET /admin", site.LoginPage("admin/index.html"))
	mux.HandleFunc("GET /admin/dashboard", site.AdminPage("admin/dashboard.html"))
	mux.HandleFunc("GET /admin/products", site.AdminPage("admin/products.html"))
	mux.HandleFunc("GET /admin/products/new", site.AdminPage("admin/product-form.html"))
	mux.HandleFunc("GET /admin/products/{id}/edit", site.AdminPage("admin/product-form.html"))
	mux.HandleFunc("GET /admin/categories", site.AdminPage("admin/categories.html"))
	mux.HandleFunc("GET /admin/gallery", site.AdminPage("admin/gallery.html"))
	mux.HandleFunc("GET /admin/company", site.AdminPage("admin/company.html"))
	mux.HandleFunc("GET /admin/settings", site.AdminPage("admin/settings.html"))

	// Публичные страницы с «красивыми» адресами.
	mux.HandleFunc("GET /catalog", site.Page("catalog.html"))
	mux.HandleFunc("GET /product/{id}", site.Page("product.html"))

	// Остальные страницы тоже без .html. Раньше красивый адрес был только
	// у каталога и карточки: /about отдавал 404, хотя /catalog работал.
	// Ссылка из письма или визитки чаще выглядит как /contacts.
	for _, name := range []string{
		"about", "delivery", "contacts", "partnership", "privacy", "sitemap",
	} {
		mux.HandleFunc("GET /"+name, site.Page(name+".html"))
	}

	mux.Handle("/", site)

	// ---------------------------------------------------------------- запуск

	srv := &http.Server{
		Addr:              *addr,
		Handler:           securityHeaders(*hsts, logRequests(gzipResponses(mux))),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second, // загрузка фотографий бывает долгой
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	// Раз в час подчищаем просроченные сессии.
	stopCleaner := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				logError(store.CleanExpiredSessions())
			case <-stopCleaner:
				return
			}
		}
	}()

	go func() {
		log.Printf("WOODWERK: слушаю %s, каталог %s", *addr, root)
		log.Printf("база данных: %s", *dbPath)
		log.Printf("фотографии: %s", *uploadsDir)
		log.Printf("заявки пишутся в %s", *leadsPath)
		log.Printf("админ-панель: /admin")
		go warnIfPortHijacked(*addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("сервер упал: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	close(stopCleaner)

	log.Println("останавливаюсь…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("принудительная остановка: %v", err)
	}
}

// resolveSiteDir находит каталог сайта — тот, где лежит index.html.
//
// Если каталог задали флагом -dir, берём только его. Иначе смотрим текущую
// папку, а потом поднимаемся вверх от самой программы: при запуске двойным
// щелчком рабочей папкой оказывается server/build, и сайт надо ещё найти.
func resolveSiteDir(dir string) (string, error) {
	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "dir" {
			explicit = true
		}
	})

	var tried []string
	look := func(path string) (string, bool) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", false
		}
		tried = append(tried, abs)
		if _, err := os.Stat(filepath.Join(abs, "index.html")); err == nil {
			return abs, true
		}
		return "", false
	}

	if found, ok := look(dir); ok {
		return found, nil
	}
	if explicit {
		return "", fmt.Errorf("в каталоге %s нет index.html — это не папка сайта", tried[0])
	}

	// Поднимаемся от программы вверх: build -> server -> корень проекта.
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		for i := 0; i < 6; i++ {
			if found, ok := look(d); ok {
				return found, nil
			}
			parent := filepath.Dir(d)
			if parent == d {
				break
			}
			d = parent
		}
	}

	return "", fmt.Errorf("не нашёл файлы сайта — папку с index.html.\nИскал здесь:\n  %s\n"+
		"Положите программу внутрь папки сайта или укажите её флагом -dir",
		strings.Join(tried, "\n  "))
}

// warnIfPortHijacked проверяет, что по нашему адресу отвечаем действительно мы.
//
// Windows разрешает занять один порт дважды, если адреса привязки разные:
// например Apache слушает 127.0.0.1:8080, а мы — все интерфейсы. Привязка
// проходит без ошибки, но запросы из браузера достаются чужой программе,
// и человек видит её страницу «404 Not Found», не понимая, что случилось.
func warnIfPortHijacked(addr string) {
	time.Sleep(700 * time.Millisecond)

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return
	}
	if host == "" || host == "0.0.0.0" || host == "[::]" {
		host = "127.0.0.1"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/api/categories")
	if err != nil {
		return // сеть недоступна — молчим, это не наш случай
	}
	defer resp.Body.Close()

	// Читаем ответ целиком: ключи в JSON идут по алфавиту, поэтому "ok"
	// стоит после списка категорий — в первые сотни байт он не попадает.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err == nil && resp.StatusCode == http.StatusOK && strings.Contains(string(body), `"ok":true`) {
		return // отвечаем мы, всё в порядке
	}

	log.Printf("")
	log.Printf("ВНИМАНИЕ: порт %s занят другой программой — она и отвечает браузеру.", port)
	log.Printf("Обычно это Apache из XAMPP, OpenServer или PostgreSQL.")
	log.Printf("Запустите сервер на свободном порту, например:")
	log.Printf("    woodwerk-windows-amd64.exe -addr :8090")
	log.Printf("и откройте http://127.0.0.1:8090/admin")
	log.Printf("")
}

// fatal печатает ошибку и не даёт окну закрыться, пока её не прочитают:
// при запуске двойным щелчком иначе видно только вспышку.
func fatal(format string, args ...any) {
	log.Printf(format, args...)
	waitForEnter()
	os.Exit(1)
}

// waitForEnter ждёт Enter, только если программа работает в живой консоли.
// Под systemd и при перенаправлении вывода ждать нельзя — сервис зависнет.
func waitForEnter() {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return
	}
	fmt.Fprint(os.Stderr, "\nНажмите Enter, чтобы закрыть окно.\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

// defaultAdminLogin и defaultAdminPassword — то, с чем панель открывается
// сразу после установки. Пароль умышленно простой, чтобы владелец не искал
// его в журнале; панель будет напоминать, что его надо сменить.
const (
	defaultAdminLogin    = "admin"
	defaultAdminPassword = "admin"
)

// ensureAdmin создаёт учётную запись при первом запуске и сообщает,
// стоит ли на ней пароль по умолчанию. Пароля нет и не может быть
// в коде фронтенда — он живёт только в базе, и только в виде хеша.
func ensureAdmin(store *Store, username, password string, reset bool) (bool, error) {
	if username == "" {
		username = os.Getenv("ADMIN_USER")
	}
	if password == "" {
		password = os.Getenv("ADMIN_PASSWORD")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = defaultAdminLogin
	}

	// Пароль задали явно — он и главнее, и требования к нему строже.
	if password != "" && len([]rune(password)) < 8 {
		return false, errors.New("пароль администратора должен быть не короче 8 символов")
	}

	user, _, err := store.UserByName(username)
	switch {
	case errors.Is(err, ErrNotFound):
		pass := password
		if pass == "" {
			pass = defaultAdminPassword
		}
		if _, err := store.CreateUser(username, pass); err != nil {
			return false, err
		}
		if pass == defaultAdminPassword {
			log.Printf("создана учётная запись администратора: логин %q, пароль %q",
				username, pass)
			warnDefaultPassword()
		} else {
			// Свой пароль в журнал не пишем: journalctl читают не только вы.
			log.Printf("создана учётная запись администратора: логин %q, пароль задан при запуске",
				username)
		}
		return pass == defaultAdminPassword, nil

	case err != nil:
		return false, err
	}

	// Учётная запись уже есть.
	switch {
	case password != "":
		if err := store.SetPassword(user.ID, password); err != nil {
			return false, err
		}
		log.Printf("пароль администратора %q обновлён из настроек запуска", username)
		return false, nil

	case reset:
		if err := store.SetPassword(user.ID, defaultAdminPassword); err != nil {
			return false, err
		}
		log.Printf("пароль администратора %q сброшен к %q", username, defaultAdminPassword)
		warnDefaultPassword()
		return true, nil
	}

	// Пароль не трогаем, но проверяем, не остался ли он стандартным.
	_, hash, err := store.UserByName(username)
	if err != nil {
		return false, err
	}
	if verifyPassword(defaultAdminPassword, hash) {
		warnDefaultPassword()
		return true, nil
	}
	return false, nil
}

func warnDefaultPassword() {
	log.Printf("")
	log.Printf("ВНИМАНИЕ: у админ-панели стандартный пароль admin/admin.")
	log.Printf("Пока он не изменён, войти может кто угодно, кто знает адрес /admin.")
	log.Printf("Смените его в панели: Настройки -> Смена пароля.")
	log.Printf("")
}
