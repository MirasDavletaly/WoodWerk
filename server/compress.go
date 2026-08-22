// Сжатие ответов.
//
// HTML, CSS, JavaScript и JSON сжимаются в 4–5 раз: главная страница
// весит 81 КБ, а по проводу уходит 17 КБ. Картинки и шрифты уже сжаты
// своими форматами — их трогать бессмысленно, только процессор греть.
package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// minCompress — ниже этого порога сжатие не окупается: заголовки и служебные
// байты gzip съедают выигрыш, а на очень мелких файлах ответ даже растёт.
const minCompress = 1024

// compressible перечисляет типы, которые имеет смысл сжимать.
func compressible(contentType string) bool {
	if i := strings.IndexByte(contentType, ';'); i > 0 {
		contentType = contentType[:i]
	}
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case "text/html", "text/css", "text/plain", "text/xml",
		"application/javascript", "text/javascript",
		"application/json", "application/xml",
		"image/svg+xml":
		return true
	}
	return false
}

// Уровень выбран средний: сжатие идёт на каждый запрос, а разница
// в размере между 6 и 9 — единицы процентов.
var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

type gzipWriter struct {
	http.ResponseWriter
	gz       *gzip.Writer
	wrote    bool // заголовки уже отправлены
	compress bool // решение принято: сжимаем или нет
}

func (g *gzipWriter) WriteHeader(code int) {
	if g.wrote {
		return
	}
	g.wrote = true

	h := g.Header()

	// Решаем по типу и размеру. Размер известен не всегда: при неизвестной
	// длине сжимаем, если тип подходящий, — так поступают все веб-серверы.
	tooSmall := false
	if cl := h.Get("Content-Length"); cl != "" {
		if n, err := parseLength(cl); err == nil && n < minCompress {
			tooSmall = true
		}
	}

	// 204 и 304 тела не несут, сжимать нечего.
	noBody := code == http.StatusNoContent || code == http.StatusNotModified

	g.compress = compressible(h.Get("Content-Type")) && !tooSmall && !noBody &&
		h.Get("Content-Encoding") == ""

	if g.compress {
		h.Set("Content-Encoding", "gzip")
		// Длина после сжатия другая, а поточно её не узнать.
		h.Del("Content-Length")
		h.Del("Accept-Ranges")
		g.gz = gzipPool.Get().(*gzip.Writer)
		g.gz.Reset(g.ResponseWriter)
	}

	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.wrote {
		// Тип содержимого мог быть определён по первым байтам.
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.compress {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// Flush нужен, чтобы обёртка не ломала потоковую отдачу.
func (g *gzipWriter) Flush() {
	if g.compress && g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipWriter) close() {
	if g.compress && g.gz != nil {
		_ = g.gz.Close()
		gzipPool.Put(g.gz)
		g.gz = nil
	}
}

// gzipResponses сжимает ответы тем клиентам, которые это умеют.
func gzipResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		// Запрос куска файла и сжатие вместе не работают: браузер ждёт
		// байты по исходным смещениям.
		if r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Accept-Encoding")

		gw := &gzipWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

func parseLength(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errBadLength
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

var errBadLength = &lengthError{}

type lengthError struct{}

func (e *lengthError) Error() string { return "некорректный Content-Length" }
