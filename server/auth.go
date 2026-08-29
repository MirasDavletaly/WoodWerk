// Авторизация администратора.
//
// Пароль нигде не хранится в открытом виде и никогда не попадает во фронтенд:
// в базе лежит только PBKDF2-HMAC-SHA256 с личной солью. Сессия — случайный
// токен в HttpOnly-cookie, в базе от него хранится лишь SHA-256.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie = "ww_admin"
	sessionTTL    = 12 * time.Hour
	pbkdf2Iters   = 210000
	pbkdf2KeyLen  = 32
)

// AdminUser — учётная запись администратора.
type AdminUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}

// Session — активная сессия администратора.
type Session struct {
	TokenHash string
	UserID    int64
	CSRF      string
	ExpiresAt time.Time
}

// ------------------------------------------------------------------ пароли

// pbkdf2SHA256 — реализация PBKDF2 (RFC 2898) на стандартной библиотеке.
// Своя, чтобы не тащить зависимость ради двадцати строк.
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	h := hmac.New(sha256.New, password)
	hashLen := h.Size()
	blocks := (keyLen + hashLen - 1) / hashLen

	out := make([]byte, 0, blocks*hashLen)
	buf := make([]byte, 4)
	for block := 1; block <= blocks; block++ {
		h.Reset()
		h.Write(salt)
		binary.BigEndian.PutUint32(buf, uint32(block))
		h.Write(buf)
		u := h.Sum(nil)

		t := make([]byte, hashLen)
		copy(t, u)
		for i := 1; i < iter; i++ {
			h.Reset()
			h.Write(u)
			u = h.Sum(u[:0])
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

// hashPassword возвращает строку вида pbkdf2$sha256$210000$<соль>$<хеш>.
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := pbkdf2SHA256([]byte(password), salt, pbkdf2Iters, pbkdf2KeyLen)
	return strings.Join([]string{
		"pbkdf2", "sha256", strconv.Itoa(pbkdf2Iters),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	}, "$"), nil
}

// verifyPassword сравнивает пароль с сохранённым хешем за постоянное время.
func verifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 5 || parts[0] != "pbkdf2" || parts[1] != "sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[2])
	if err != nil || iter < 1000 || iter > 5000000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	got := pbkdf2SHA256([]byte(password), salt, iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ------------------------------------------------------------------ пользователи

func (s *Store) UserByName(username string) (*AdminUser, string, error) {
	var u AdminUser
	var hash string
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM admin_users WHERE username = ?`,
		username).Scan(&u.ID, &u.Username, &hash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

func (s *Store) UserByID(id int64) (*AdminUser, string, error) {
	var u AdminUser
	var hash string
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM admin_users WHERE id = ?`,
		id).Scan(&u.ID, &u.Username, &hash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, password string) (*AdminUser, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	ts := now()
	res, err := s.db.Exec(
		`INSERT INTO admin_users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		username, hash, ts, ts)
	if isUnique(err) {
		return nil, ErrDuplicate
	}
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	u, _, err := s.UserByID(id)
	return u, err
}

// SetPassword меняет пароль и гасит все сессии, кроме текущей.
func (s *Store) SetPassword(userID int64, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE admin_users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, now(), userID)
	return err
}

// SetUsername меняет логин. Логин уникален, поэтому занятое имя
// возвращает ErrDuplicate, а не молча ничего не делает.
func (s *Store) SetUsername(userID int64, username string) error {
	_, err := s.db.Exec(`UPDATE admin_users SET username = ?, updated_at = ? WHERE id = ?`,
		username, now(), userID)
	if isUnique(err) {
		return ErrDuplicate
	}
	return err
}

// ------------------------------------------------------------------ сессии

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession выдаёт токен для cookie и парный CSRF-токен для заголовка.
func (s *Store) CreateSession(userID int64) (token, csrf string, err error) {
	if token, err = randomToken(32); err != nil {
		return "", "", err
	}
	if csrf, err = randomToken(24); err != nil {
		return "", "", err
	}
	expires := time.Now().UTC().Add(sessionTTL)
	_, err = s.db.Exec(`
        INSERT INTO sessions (token_hash, user_id, csrf, created_at, expires_at)
        VALUES (?, ?, ?, ?, ?)`,
		hashToken(token), userID, csrf, now(), expires.Format(time.RFC3339))
	if err != nil {
		return "", "", err
	}
	return token, csrf, nil
}

func (s *Store) Session(token string) (*Session, error) {
	var sess Session
	var expires string
	err := s.db.QueryRow(
		`SELECT token_hash, user_id, csrf, expires_at FROM sessions WHERE token_hash = ?`,
		hashToken(token)).Scan(&sess.TokenHash, &sess.UserID, &sess.CSRF, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sess.ExpiresAt, err = time.Parse(time.RFC3339, expires)
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, sess.TokenHash)
		return nil, ErrNotFound
	}
	return &sess, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

// DeleteUserSessions гасит все сессии пользователя, кроме указанной.
func (s *Store) DeleteUserSessions(userID int64, keepToken string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`,
		userID, hashToken(keepToken))
	return err
}

func (s *Store) CleanExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now())
	return err
}

// ------------------------------------------------------------------ HTTP-слой

// Auth связывает HTTP-запросы с сессиями в базе.
type Auth struct {
	store  *Store
	secure bool // ставить ли флаг Secure у cookie
}

func NewAuth(store *Store, secure bool) *Auth {
	return &Auth{store: store, secure: secure}
}

func (a *Auth) setCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true, // JavaScript до токена не добирается
		Secure:   a.secure || r.TLS != nil || forwardedHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
		MaxAge:   int(sessionTTL / time.Second),
	})
}

func (a *Auth) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure || r.TLS != nil || forwardedHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// current достаёт сессию и пользователя из запроса.
func (a *Auth) current(r *http.Request) (*Session, *AdminUser, string) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil, nil, ""
	}
	sess, err := a.store.Session(c.Value)
	if err != nil {
		return nil, nil, ""
	}
	user, _, err := a.store.UserByID(sess.UserID)
	if err != nil {
		return nil, nil, ""
	}
	return sess, user, c.Value
}

// LoggedIn — быстрая проверка для страниц админки.
func (a *Auth) LoggedIn(r *http.Request) bool {
	_, user, _ := a.current(r)
	return user != nil
}

// ctxKey — свой тип, чтобы ключ контекста ни с чем не столкнулся.
type ctxKey string

const (
	ctxUser    ctxKey = "admin_user"
	ctxSession ctxKey = "admin_session"
	ctxToken   ctxKey = "admin_token"
)

// Protect закрывает API админки: без сессии — 401, без CSRF-заголовка
// на изменяющем запросе — 403.
func (a *Auth) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, user, token := a.current(r)
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, apiError("Требуется вход в админ-панель"))
			return
		}

		// Cookie SameSite=Lax уже отсекает межсайтовые POST, но заголовок,
		// который умеет ставить только наш скрипт, — вторая линия защиты.
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(sess.CSRF)) != 1 {
				writeJSON(w, http.StatusForbidden, apiError("Сессия устарела. Войдите заново."))
				return
			}
		}

		ctx := r.Context()
		ctx = contextWith(ctx, ctxUser, user)
		ctx = contextWith(ctx, ctxSession, sess)
		ctx = contextWith(ctx, ctxToken, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
