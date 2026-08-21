package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

// apiError — единый формат ошибки для фронтенда. Текст всегда русский:
// он показывается пользователю как есть.
func apiError(message string) map[string]any {
	return map[string]any{"ok": false, "error": message}
}

func apiOK(payload map[string]any) map[string]any {
	out := map[string]any{"ok": true}
	for k, v := range payload {
		out[k] = v
	}
	return out
}

func contextWith(ctx context.Context, key ctxKey, value any) context.Context {
	return context.WithValue(ctx, key, value)
}

func userFrom(r *http.Request) *AdminUser {
	u, _ := r.Context().Value(ctxUser).(*AdminUser)
	return u
}

func sessionFrom(r *http.Request) *Session {
	s, _ := r.Context().Value(ctxSession).(*Session)
	return s
}

func tokenFrom(r *http.Request) string {
	t, _ := r.Context().Value(ctxToken).(string)
	return t
}

// pathID читает {id} из адреса.
func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// translit — таблица для превращения русских названий в латинские слаги.
var translit = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "h", 'ц': "c", 'ч': "ch", 'ш': "sh", 'щ': "sch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// slugify делает из «Кухонная мебель» адресную строку kuhonnaya-mebel.
func slugify(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case translit[r] != "":
			b.WriteString(translit[r])
		case r == 'ъ' || r == 'ь':
			// пропускаем
		default:
			b.WriteByte('-')
		}
	}

	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-")
	}
	return out
}

// clean убирает управляющие символы и режет строку до максимума рун.
func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = strings.TrimSpace(string(r[:max]))
	}
	return s
}

// cleanLine — то же, но ещё и без переводов строки (для однострочных полей).
func cleanLine(s string, max int) string {
	return clean(strings.ReplaceAll(s, "\n", " "), max)
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
