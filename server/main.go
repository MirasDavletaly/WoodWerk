// Небольшой сервер для сайта WOODWERK.
//
// Делает две вещи, которых не умеет статический хостинг вроде GitHub Pages:
//   1. отдаёт заголовки безопасности (HSTS, X-Frame-Options, CSP и прочее);
//   2. принимает заявки с форм на POST /api/lead и складывает их в JSONL.
//
// Только стандартная библиотека, без зависимостей.
//
//	go run ./server -addr :8080 -dir .
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
)

const contentSecurityPolicy = "default-src 'self'; " +
	"base-uri 'self'; " +
	"object-src 'none'; " +
	"script-src 'self'; " +
	"style-src 'self' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"img-src 'self' data:; " +
	"frame-src https://yandex.ru https://*.yandex.ru; " +
	"connect-src 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"upgrade-insecure-requests"

// configJS подменяет assets/js/config.js: при работе через этот сервер
// фронтенд должен слать заявки на /api/lead.
const configJS = `/* Подставлено Go-сервером woodwerk. */
window.WOODWERK = { leadEndpoint: "/api/lead" };
`

func main() {
	addr := flag.String("addr", ":8080", "адрес и порт, например :8080")
	dir := flag.String("dir", ".", "каталог с файлами сайта")
	leadsPath := flag.String("leads", "leads.jsonl", "файл, куда дописываются заявки")
	hsts := flag.Bool("hsts", false, "слать Strict-Transport-Security (только когда сайт реально за HTTPS)")
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("некорректный каталог: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); err != nil {
		log.Fatalf("в каталоге %s нет index.html", root)
	}

	store, err := newLeadStore(*leadsPath)
	if err != nil {
		log.Fatalf("не открыть файл заявок: %v", err)
	}
	defer store.Close()

	mux := http.NewServeMux()
	mux.Handle("/api/lead", &leadHandler{store: store, limiter: newLimiter()})
	mux.Handle("/", staticHandler(root))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           securityHeaders(*hsts, logRequests(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Printf("WOODWERK: слушаю %s, каталог %s", *addr, root)
		log.Printf("заявки пишутся в %s", *leadsPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("сервер упал: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("останавливаюсь…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("принудительная остановка: %v", err)
	}
}

// ---------------------------------------------------------------- статика

func staticHandler(root string) http.Handler {
	files := http.FileServer(http.Dir(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)

		// Конфиг фронтенда отдаём свой, с включённым эндпоинтом.
		if clean == "/assets/js/config.js" {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(configJS))
			return
		}

		// Прячем то, что не должно быть доступно снаружи.
		for _, part := range strings.Split(clean, "/") {
			if strings.HasPrefix(part, ".") && part != "." && part != ".well-known" {
				http.NotFound(w, r)
				return
			}
		}
		switch strings.ToLower(filepath.Ext(clean)) {
		case ".md", ".py", ".go", ".mod", ".log", ".bak":
			http.NotFound(w, r)
			return
		}

		if strings.HasSuffix(clean, ".html") || clean == "/" {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		} else if strings.HasPrefix(clean, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		files.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------- заголовки

func securityHeaders(hsts bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "accelerometer=(), autoplay=(), camera=(), display-capture=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// ---------------------------------------------------------------- заявки

type lead struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Type    string `json:"type,omitempty"`
	Comment string `json:"comment,omitempty"`
	Company string `json:"company,omitempty"` // honeypot: у человека всегда пусто
}

type storedLead struct {
	lead
	At        string `json:"at"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
}

type leadStore struct {
	mu sync.Mutex
	f  *os.File
}

func newLeadStore(path string) (*leadStore, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &leadStore{f: f}, nil
}

func (s *leadStore) Append(item storedLead) error {
	line, err := json.Marshal(item)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.f.Write(append(line, '\n')); err != nil {
		return err
	}
	return s.f.Sync()
}

func (s *leadStore) Close() error { return s.f.Close() }

type leadHandler struct {
	store   *leadStore
	limiter *limiter
}

func (h *leadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "только POST"})
		return
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"ok": false, "error": "нужен application/json"})
		return
	}

	ip := clientIP(r)
	if !h.limiter.allow(ip, 5, 10*time.Minute) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "слишком много заявок, попробуйте позже"})
		return
	}

	var in lead
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	if err := dec.Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "некорректный JSON"})
		return
	}

	// Ловушка для ботов: поле скрыто от людей, заполнить его может только скрипт.
	// Отвечаем как при успехе, чтобы бот не понял, что его отсеяли.
	if strings.TrimSpace(in.Company) != "" {
		log.Printf("honeypot сработал, ip=%s", ip)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	in.Name = clean(in.Name, 80)
	in.Phone = clean(in.Phone, 32)
	in.Type = clean(in.Type, 80)
	in.Comment = clean(in.Comment, 1000)

	if n := len([]rune(in.Name)); n < 2 || n > 80 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "проверьте имя"})
		return
	}
	if digitsOnly(in.Phone) != 11 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "проверьте телефон"})
		return
	}

	item := storedLead{
		lead:      lead{Name: in.Name, Phone: in.Phone, Type: in.Type, Comment: in.Comment},
		At:        time.Now().Format(time.RFC3339),
		IP:        ip,
		UserAgent: clean(r.UserAgent(), 200),
	}
	if err := h.store.Append(item); err != nil {
		log.Printf("не записать заявку: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "внутренняя ошибка"})
		return
	}

	log.Printf("новая заявка: %s, %s", item.Name, item.Phone)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// clean убирает управляющие символы и режет строку до максимума рун.
func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

func digitsOnly(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			n++
		}
	}
	return n
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------------------------------------------------------------- лимит частоты

type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newLimiter() *limiter {
	return &limiter{hits: make(map[string][]time.Time)}
}

// allow пропускает не больше max обращений с одного адреса за window.
func (l *limiter) allow(key string, max int, window time.Duration) bool {
	now := time.Now()
	cutoff := now.Add(-window)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Заодно подчищаем протухшие записи, чтобы карта не росла бесконечно.
	for k, times := range l.hits {
		kept := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.hits, k)
		} else {
			l.hits[k] = kept
		}
	}

	if len(l.hits[key]) >= max {
		return false
	}
	l.hits[key] = append(l.hits[key], now)
	return true
}
