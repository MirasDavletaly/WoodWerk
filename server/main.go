// Сервер сайта WOODWERK.
//
// Делает то, чего не умеет статический хостинг:
//  1. отдаёт заголовки безопасности (HSTS, X-Frame-Options, CSP и прочее);
//  2. принимает заявки с форм на POST /api/lead и складывает их в JSONL;
//  3. хранит каталог мебели в SQLite и отдаёт его сайту через /api/…;
//  4. поднимает админ-панель на /admin с авторизацией и загрузкой фотографий.
//
// Запуск:
//
//	go run ./server -addr :8080 -dir .
//
// При первом запуске создаётся учётная запись администратора; логин и пароль
// печатаются в журнал. Задать их заранее можно через ADMIN_USER и
// ADMIN_PASSWORD или флаги -admin-user и -admin-pass.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", ":8080", "адрес и порт, например :8080")
	dir := flag.String("dir", ".", "каталог с файлами сайта")
	leadsPath := flag.String("leads", "leads.jsonl", "файл, куда дописываются заявки")
	dbPath := flag.String("db", "data/woodwerk.db", "файл базы данных SQLite")
	uploadsDir := flag.String("uploads", "uploads", "каталог для загруженных фотографий")
	maxUpload := flag.Int("max-upload", 5, "предельный размер одной фотографии, МБ")
	hsts := flag.Bool("hsts", false, "слать Strict-Transport-Security (только когда сайт реально за HTTPS)")
	adminUser := flag.String("admin-user", "", "логин администратора (по умолчанию admin)")
	adminPass := flag.String("admin-pass", "", "пароль администратора; на уже созданной учётной записи меняет пароль")
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("некорректный каталог: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); err != nil {
		log.Fatalf("в каталоге %s нет index.html", root)
	}

	// ---------------------------------------------------------------- база

	db, err := openDB(*dbPath)
	if err != nil {
		log.Fatalf("не открыть базу данных: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Seed(); err != nil {
		log.Fatalf("не заполнить базу: %v", err)
	}
	if err := ensureAdmin(store, *adminUser, *adminPass); err != nil {
		log.Fatalf("не создать администратора: %v", err)
	}
	if err := store.CleanExpiredSessions(); err != nil {
		logError(err)
	}

	uploads, err := NewUploads(*uploadsDir, *maxUpload)
	if err != nil {
		log.Fatalf("не создать каталог загрузок: %v", err)
	}

	leads, err := newLeadStore(*leadsPath)
	if err != nil {
		log.Fatalf("не открыть файл заявок: %v", err)
	}
	defer leads.Close()

	// ---------------------------------------------------------------- маршруты

	auth := NewAuth(store, *hsts)
	api := &API{store: store, uploads: uploads, auth: auth, logins: newLimiter()}
	site := NewSite(root, auth)

	mux := http.NewServeMux()

	// Заявки с форм — как и раньше.
	mux.Handle("/api/lead", &leadHandler{store: leads, limiter: newLimiter()})

	// Публичное API: только активные изделия, только чтение.
	mux.HandleFunc("GET /api/products", api.publicProducts)
	mux.HandleFunc("GET /api/products/{id}", api.publicProduct)
	mux.HandleFunc("GET /api/categories", api.publicCategories)

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
	admin.HandleFunc("POST /api/admin/upload", api.upload)
	admin.HandleFunc("POST /api/admin/upload/delete", api.deleteUpload)
	admin.HandleFunc("POST /api/admin/password", api.changePassword)
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
	mux.HandleFunc("GET /admin/settings", site.AdminPage("admin/settings.html"))

	// Публичные страницы с «красивыми» адресами.
	mux.HandleFunc("GET /catalog", site.Page("catalog.html"))
	mux.HandleFunc("GET /product/{id}", site.Page("product.html"))

	mux.Handle("/", site)

	// ---------------------------------------------------------------- запуск

	srv := &http.Server{
		Addr:              *addr,
		Handler:           securityHeaders(*hsts, logRequests(mux)),
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
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("сервер упал: %v", err)
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

// ensureAdmin создаёт учётную запись при первом запуске. Пароль берётся
// из флага или переменной окружения, иначе генерируется и печатается в журнал.
// В коде фронтенда пароля нет и быть не может.
func ensureAdmin(store *Store, username, password string) error {
	if username == "" {
		username = os.Getenv("ADMIN_USER")
	}
	if password == "" {
		password = os.Getenv("ADMIN_PASSWORD")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}

	user, _, err := store.UserByName(username)
	switch {
	case errors.Is(err, ErrNotFound):
		generated := false
		if password == "" {
			if password, err = randomToken(9); err != nil {
				return err
			}
			generated = true
		}
		if len([]rune(password)) < 8 {
			return errors.New("пароль администратора должен быть не короче 8 символов")
		}
		if _, err := store.CreateUser(username, password); err != nil {
			return err
		}
		log.Printf("создана учётная запись администратора: логин %q", username)
		if generated {
			log.Printf("ПАРОЛЬ (показывается один раз, сохраните его): %s", password)
		}
		return nil

	case err != nil:
		return err
	}

	// Учётная запись уже есть: пароль меняем, только если его задали явно.
	if password != "" {
		if len([]rune(password)) < 8 {
			return errors.New("пароль администратора должен быть не короче 8 символов")
		}
		if err := store.SetPassword(user.ID, password); err != nil {
			return err
		}
		log.Printf("пароль администратора %q обновлён из настроек запуска", username)
	}
	return nil
}
