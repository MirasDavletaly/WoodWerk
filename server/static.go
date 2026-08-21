// Отдача файлов сайта и «человеческие» адреса вида /catalog, /product/12,
// /admin/dashboard — без .html в конце.
package main

import (
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// configJS подменяет assets/js/config.js: при работе через этот сервер
// фронтенд знает и адрес заявок, и адрес API каталога.
const configJS = `/* Подставлено Go-сервером woodwerk. */
window.WOODWERK = { leadEndpoint: "/api/lead", apiBase: "/api" };
`

// Site отдаёт статику из каталога сайта.
type Site struct {
	root  string
	files http.Handler
	auth  *Auth
}

func NewSite(root string, auth *Auth) *Site {
	return &Site{root: root, files: http.FileServer(http.Dir(root)), auth: auth}
}

// ServeHTTP — обычная раздача файлов для всего, что не попало в явные маршруты.
func (s *Site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean("/" + r.URL.Path)

	// Конфиг фронтенда отдаём свой, с включёнными эндпоинтами.
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
	case ".md", ".py", ".go", ".mod", ".sum", ".log", ".bak", ".db", ".jsonl":
		http.NotFound(w, r)
		return
	}
	// Каталоги с данными наружу не отдаём ни при каких обстоятельствах.
	if strings.HasPrefix(clean, "/server/") || strings.HasPrefix(clean, "/data/") {
		http.NotFound(w, r)
		return
	}

	s.cacheHeaders(w, clean)
	s.files.ServeHTTP(w, r)
}

func (s *Site) cacheHeaders(w http.ResponseWriter, clean string) {
	switch {
	case strings.HasSuffix(clean, ".html"), clean == "/":
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	case strings.HasPrefix(clean, "/assets/"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
}

// Page отдаёт конкретный файл сайта по «красивому» адресу.
func (s *Site) Page(file string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.serveFile(w, r, file)
	}
}

// AdminPage — то же, но страница закрыта: без действующей сессии
// посетителя отправляем на форму входа.
func (s *Site) AdminPage(file string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.LoggedIn(r) {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		s.serveFile(w, r, file)
	}
}

// LoginPage показывает форму входа, а вошедшего сразу ведёт в панель.
func (s *Site) LoginPage(file string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.auth.LoggedIn(r) {
			http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
			return
		}
		s.serveFile(w, r, file)
	}
}

func (s *Site) serveFile(w http.ResponseWriter, r *http.Request, file string) {
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	http.ServeFile(w, r, filepath.Join(s.root, filepath.FromSlash(file)))
}
