// Отдача файлов сайта и «человеческие» адреса вида /catalog, /product/12,
// /admin/dashboard — без .html в конце.
package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// configJS подменяет assets/js/config.js: при работе через этот сервер
// фронтенд знает и адрес заявок, и адрес API каталога.
const configJS = `/* Подставлено Go-сервером woodwerk. */
window.WOODWERK = { leadEndpoint: "/api/lead", apiBase: "/api" };
`

// internalDirs — каталоги, в которых нет ничего для посетителя: исходники,
// данные, инструменты и файлы развёртывания. Последние особенно неприятны:
// они рассказывают, где что лежит на сервере и как устроена служба.
var internalDirs = []string{
	"/server/", "/data/", "/deploy/", "/tools/", "/node_modules/",
}

// internalExts — расширения, которых на публичном сайте быть не должно.
var internalExts = map[string]bool{
	".md": true, ".py": true, ".go": true, ".mod": true, ".sum": true,
	".log": true, ".bak": true, ".db": true, ".jsonl": true,
	".sh": true, ".bat": true, ".ps1": true, ".conf": true, ".service": true,
	".env": true, ".ini": true, ".yml": true, ".yaml": true, ".sql": true,
}

// publicPath решает, можно ли отдавать файл наружу.
func publicPath(clean string) bool {
	lower := strings.ToLower(clean)
	for _, dir := range internalDirs {
		if strings.HasPrefix(lower, dir) {
			return false
		}
	}
	return !internalExts[strings.ToLower(filepath.Ext(clean))]
}

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
			s.NotFound(w, r)
			return
		}
	}
	if !publicPath(clean) {
		s.NotFound(w, r)
		return
	}

	// Существует ли файл, надо знать заранее: http.FileServer на отсутствие
	// отвечает своей страницей, и подменить её потом уже нельзя.
	if !s.exists(clean) {
		s.NotFound(w, r)
		return
	}

	s.cacheHeaders(w, clean)
	s.files.ServeHTTP(w, r)
}

// exists проверяет, что по адресу лежит именно файл. Каталог не в счёт:
// листинг папок наружу не отдаём.
func (s *Site) exists(clean string) bool {
	if clean == "/" {
		clean = "/index.html"
	}
	info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(clean)))
	return err == nil && !info.IsDir()
}

// NotFound отдаёт страницу сайта вместо строчки «404 page not found».
// Посетитель, набравший адрес с опечаткой, должен увидеть меню и дорогу
// в каталог, а не голый текст на белом фоне.
func (s *Site) NotFound(w http.ResponseWriter, r *http.Request) {
	page := filepath.Join(s.root, "404.html")
	body, err := os.ReadFile(page)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func (s *Site) cacheHeaders(w http.ResponseWriter, clean string) {
	switch {
	case strings.HasSuffix(clean, ".html"), clean == "/":
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	case strings.HasPrefix(clean, "/assets/i18n/"):
		// Словари переводов правятся без переименования файла, поэтому
		// вечный кэш здесь означал бы, что правка не дойдёт до посетителя.
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
