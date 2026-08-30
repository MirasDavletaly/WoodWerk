// Приём заявок с форм сайта. Заявки дописываются в JSONL-файл —
// так же, как и до появления админ-панели.
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

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
	mail    *mailer
}

func (h *leadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, apiError("только POST"))
		return
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, apiError("нужен application/json"))
		return
	}

	ip := clientIP(r)
	if !h.limiter.allow(ip, 5, 10*time.Minute) {
		writeJSON(w, http.StatusTooManyRequests, apiError("слишком много заявок, попробуйте позже"))
		return
	}

	var in lead
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	if err := dec.Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError("некорректный JSON"))
		return
	}

	// Ловушка для ботов: поле скрыто от людей, заполнить его может только скрипт.
	// Отвечаем как при успехе, чтобы бот не понял, что его отсеяли.
	if strings.TrimSpace(in.Company) != "" {
		logInfo("honeypot сработал, ip=%s", ip)
		writeJSON(w, http.StatusOK, apiOK(nil))
		return
	}

	in.Name = cleanLine(in.Name, 80)
	in.Phone = cleanLine(in.Phone, 32)
	in.Type = cleanLine(in.Type, 80)
	in.Comment = clean(in.Comment, 1000)

	if n := len([]rune(in.Name)); n < 2 || n > 80 {
		writeJSON(w, http.StatusBadRequest, apiError("проверьте имя"))
		return
	}
	if digitsOnly(in.Phone) != 11 {
		writeJSON(w, http.StatusBadRequest, apiError("проверьте телефон"))
		return
	}

	item := storedLead{
		lead:      lead{Name: in.Name, Phone: in.Phone, Type: in.Type, Comment: in.Comment},
		At:        time.Now().Format(time.RFC3339),
		IP:        ip,
		UserAgent: cleanLine(r.UserAgent(), 200),
	}
	if err := h.store.Append(item); err != nil {
		logError(err)
		writeJSON(w, http.StatusInternalServerError, apiError("внутренняя ошибка"))
		return
	}

	// Письмо отправляем отдельно и не ждём его: SMTP отвечает не мгновенно,
	// а заявка уже сохранена — посетителю ждать нечего.
	go h.mail.Send(item)

	logInfo("новая заявка: %s, %s", item.Name, item.Phone)
	writeJSON(w, http.StatusOK, apiOK(nil))
}
