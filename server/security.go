// Заголовки безопасности, журнал запросов и ограничение частоты обращений.
package main

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// contentSecurityPolicy запрещает подключать чужие скрипты и кадры.
// blob: в img-src нужен для предпросмотра фотографии в админке до загрузки.
const contentSecurityPolicy = "default-src 'self'; " +
	"base-uri 'self'; " +
	"object-src 'none'; " +
	"script-src 'self'; " +
	"style-src 'self' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"img-src 'self' data: blob:; " +
	"frame-src https://yandex.ru https://*.yandex.ru; " +
	"connect-src 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"upgrade-insecure-requests"

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

func logInfo(format string, args ...any) { log.Printf(format, args...) }

func logError(err error) {
	if err != nil {
		log.Printf("ошибка: %v", err)
	}
}

// clientIP определяет адрес посетителя.
//
// За обратным прокси соединение всегда приходит с 127.0.0.1, и если верить
// только ему, лимит попыток входа станет общим на всех: десять неудач от
// кого угодно — и владелец сайта тоже получает отказ.
//
// Заголовкам верим ровно в одном случае: когда соединение пришло с петлевого
// адреса, то есть от нашего же nginx на этой машине. Заголовок от постороннего
// — это подсказка злоумышленника, как обойти лимит, и мы её игнорируем.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if !isLoopback(host) {
		return host
	}

	// X-Real-IP наш прокси перезаписывает сам, подделать его нельзя.
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); validIP(real) {
		return real
	}

	// В X-Forwarded-For прокси дописывает адрес в конец, поэтому настоящий
	// клиент — последний в списке; всё, что левее, прислал сам клиент.
	if chain := r.Header.Get("X-Forwarded-For"); chain != "" {
		parts := strings.Split(chain, ",")
		last := strings.TrimSpace(parts[len(parts)-1])
		if validIP(last) {
			return last
		}
	}

	return host
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validIP(s string) bool {
	return s != "" && net.ParseIP(s) != nil
}

// forwardedHTTPS сообщает, что посетитель пришёл по HTTPS, а расшифровал
// соединение прокси. Заголовку верим по тому же правилу, что и адресу.
func forwardedHTTPS(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !isLoopback(host) {
		return false
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	return strings.EqualFold(strings.TrimSpace(proto), "https")
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
