package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// За обратным прокси адрес соединения всегда 127.0.0.1. Если верить только
// ему, лимит попыток входа станет общим на всех посетителей: десять неудач
// от кого угодно — и настоящий владелец получает отказ.
func TestClientIPBehindProxy(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		header map[string]string
		want   string
	}{
		{
			name:   "прямое соединение",
			remote: "203.0.113.7:51234",
			want:   "203.0.113.7",
		},
		{
			name:   "свой прокси передал настоящий адрес",
			remote: "127.0.0.1:40000",
			header: map[string]string{"X-Real-Ip": "203.0.113.7"},
			want:   "203.0.113.7",
		},
		{
			name:   "цепочка X-Forwarded-For от своего прокси",
			remote: "127.0.0.1:40000",
			header: map[string]string{"X-Forwarded-For": "198.51.100.1, 203.0.113.7"},
			want:   "203.0.113.7",
		},
		{
			// Заголовку от постороннего верить нельзя: иначе лимит обходится
			// подстановкой любого адреса в каждом запросе.
			name:   "чужой адрес подделывает заголовок",
			remote: "203.0.113.7:51234",
			header: map[string]string{"X-Real-Ip": "10.0.0.1", "X-Forwarded-For": "10.0.0.2"},
			want:   "203.0.113.7",
		},
		{
			name:   "прокси прислал мусор",
			remote: "127.0.0.1:40000",
			header: map[string]string{"X-Real-Ip": "не-адрес"},
			want:   "127.0.0.1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = c.remote
			for k, v := range c.header {
				r.Header.Set(k, v)
			}
			if got := clientIP(r); got != c.want {
				t.Errorf("clientIP = %q, ожидалось %q", got, c.want)
			}
		})
	}
}

// Служебные файлы наружу отдавать нельзя: они рассказывают, как устроен
// сервер и где что лежит.
func TestStaticHidesInternalFiles(t *testing.T) {
	root := t.TempDir()

	must := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	must("index.html", "<html></html>")
	must("assets/css/style.css", "body{}")
	must("deploy/nginx.conf", "server { listen 443; }")
	must("deploy/install.sh", "#!/bin/bash")
	must("deploy/woodwerk.service", "[Unit]")
	must("tools/i18n/extract.py", "print(1)")
	must("server/main.go", "package main")
	must("data/woodwerk.db", "SQLite")
	must("Запустить сайт.bat", "@echo off")
	must("leads.jsonl", "{}")

	site := NewSite(root, NewAuth(nil, false))

	hidden := []string{
		"/deploy/nginx.conf",
		"/deploy/install.sh",
		"/deploy/woodwerk.service",
		"/tools/i18n/extract.py",
		"/server/main.go",
		"/data/woodwerk.db",
		"/%D0%97%D0%B0%D0%BF%D1%83%D1%81%D1%82%D0%B8%D1%82%D1%8C%20%D1%81%D0%B0%D0%B9%D1%82.bat", // «Запустить сайт.bat»
		"/leads.jsonl",
	}
	for _, path := range hidden {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		site.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s отдаётся с кодом %d, а должен быть скрыт", path, w.Code)
		}
	}

	// FileServer штатно уводит /index.html на /, поэтому проверяем корень.
	public := []string{"/", "/assets/css/style.css"}
	for _, path := range public {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		site.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("%s не отдаётся (код %d), а должен", path, w.Code)
		}
	}
}

// Куку сессии нельзя отдавать без флага Secure, когда сайт работает
// по HTTPS: за прокси r.TLS пустой, и признак приходит заголовком.
func TestSessionCookieSecureBehindProxy(t *testing.T) {
	auth := NewAuth(nil, false)

	r := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	r.RemoteAddr = "127.0.0.1:40000"
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	auth.setCookie(w, r, "token")

	cookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "Secure") {
		t.Errorf("нет флага Secure за HTTPS-прокси: %s", cookie)
	}

	// А по обычному http флага быть не должно, иначе кука не доедет.
	r2 := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	r2.RemoteAddr = "127.0.0.1:40000"
	w2 := httptest.NewRecorder()
	auth.setCookie(w2, r2, "token")
	if strings.Contains(w2.Header().Get("Set-Cookie"), "Secure") {
		t.Error("флаг Secure выставлен без HTTPS")
	}
}
