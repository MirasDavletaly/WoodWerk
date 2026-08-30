// Данные компании: контакты, адреса, соцсети и логотип.
//
// Раньше они были свёрстаны прямо в разметке — телефон встречался
// в сотне мест, и смена номера означала правку десяти файлов. Теперь
// значения лежат в базе, страница подставляет их при загрузке,
// а администратор меняет их на одном экране.
package main

import (
	"database/sql"
	"net/http"
	"strings"
)

// Setting — одна настройка. Ключи заданы в коде: произвольные
// администратор не заводит, иначе разметка и база разъедутся.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// settingSpec описывает поле на экране админки.
type settingSpec struct {
	Key     string
	Label   string
	Hint    string
	Kind    string // text | tel | email | url | image
	Default string
}

// settingSpecs — единственный список, из которого берутся и значения
// по умолчанию, и поля формы. Добавить настройку — добавить строку сюда.
var settingSpecs = []settingSpec{
	{"company_name", "Название компании", "Показывается в шапке, подвале и заголовках", "text", "WOODWERK"},
	{"company_tagline", "Подпись под названием", "", "text", "стеновые панели"},
	{"logo_url", "Логотип", "SVG или PNG; заменяется загрузкой файла", "image", "/assets/img/logo.svg"},

	{"phone_main", "Телефон, основной", "", "tel", "+7 (707) 139-49-09"},
	{"phone_main_note", "Подпись к основному телефону", "Например, город или офис", "text", "Астана, ул. Шугыла 23/2, Коктал №1"},
	{"phone_extra", "Телефон, второй", "Оставьте пустым, если он один", "tel", "+7 (707) 718-20-15"},
	{"phone_extra_note", "Подпись ко второму телефону", "", "text", "Алматы, ул. Емцова 9А, 2 этаж, офис №888"},

	{"email", "Электронная почта", "", "email", "info@woodwerk.ru"},
	{"hours", "График работы", "", "text", "Пн–Пт 9:00–20:00, Сб 10:00–18:00"},

	{"address_main", "Адрес, основной", "", "text", "Астана, ул. Шугыла 23/2, Коктал №1"},
	{"address_extra", "Адрес, второй", "Оставьте пустым, если офис один", "text", "Алматы, ул. Емцова 9А, 2 этаж, офис №888"},

	{"social_whatsapp", "WhatsApp", "Полная ссылка, например https://wa.me/77071394909", "url", ""},
	{"social_telegram", "Telegram", "", "url", ""},
	{"social_instagram", "Instagram", "", "url", ""},
	{"social_vk", "ВКонтакте", "", "url", ""},
	{"social_pinterest", "Pinterest", "", "url", ""},
}

func knownSetting(key string) *settingSpec {
	for i := range settingSpecs {
		if settingSpecs[i].Key == key {
			return &settingSpecs[i]
		}
	}
	return nil
}

// ------------------------------------------------------------------ запросы

// Settings отдаёт все настройки: значения из базы, а чего в ней нет —
// значения по умолчанию. Так страница никогда не остаётся с пустотой.
func (s *Store) Settings() (map[string]string, error) {
	out := make(map[string]string, len(settingSpecs))
	for _, spec := range settingSpecs {
		out[spec.Key] = spec.Default
	}

	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if knownSetting(k) != nil {
			out[k] = v
		}
	}
	return out, rows.Err()
}

// SaveSettings записывает присланные значения. Неизвестные ключи
// молча пропускаются: список полей задаётся кодом, а не запросом.
func (s *Store) SaveSettings(values map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ts := now()
	for key, value := range values {
		if knownSetting(key) == nil {
			continue
		}
		if _, err := tx.Exec(`
            INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
            ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			key, value, ts); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SettingsUseImage сообщает, стоит ли файл логотипом: удалять его
// вместе с товаром нельзя.
func (s *Store) SettingsUseImage(url string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM settings WHERE value = ?`, url).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return n > 0, err
}

// ------------------------------------------------------------------ обработчики

// GET /api/settings — данные компании для страниц сайта.
func (a *API) publicSettings(w http.ResponseWriter, r *http.Request) {
	values, err := a.store.Settings()
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"settings": values}))
}

// GET /api/admin/settings-site — значения вместе с описанием полей,
// чтобы форма строилась по одному источнику правды.
func (a *API) adminSiteSettings(w http.ResponseWriter, r *http.Request) {
	values, err := a.store.Settings()
	if err != nil {
		serverError(w, err)
		return
	}

	fields := make([]map[string]string, 0, len(settingSpecs))
	for _, spec := range settingSpecs {
		fields = append(fields, map[string]string{
			"key": spec.Key, "label": spec.Label,
			"hint": spec.Hint, "kind": spec.Kind,
		})
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"settings": values,
		"fields":   fields,
	}))
}

// PUT /api/admin/settings-site
func (a *API) updateSiteSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Settings map[string]string `json:"settings"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Settings) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError("Нечего сохранять"))
		return
	}

	clean := make(map[string]string, len(in.Settings))
	for key, value := range in.Settings {
		spec := knownSetting(key)
		if spec == nil {
			continue
		}
		value = cleanLine(value, 300)

		switch spec.Kind {
		case "url":
			// Пустая строка допустима: значит, ссылки нет и значок прячется.
			if value != "" && !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
				writeJSON(w, http.StatusBadRequest,
					apiError("Ссылка «"+spec.Label+"» должна начинаться с https://"))
				return
			}
		case "image":
			// Только свои файлы: чужой адрес в src подгружал бы картинку
			// со стороннего сервера на каждой странице.
			if !strings.HasPrefix(value, "/uploads/") && !strings.HasPrefix(value, "/assets/") {
				writeJSON(w, http.StatusBadRequest,
					apiError("Выберите изображение, загруженное на сайт"))
				return
			}
		case "email":
			if value != "" && !strings.Contains(value, "@") {
				writeJSON(w, http.StatusBadRequest, apiError("Почта указана неверно"))
				return
			}
		}
		clean[key] = value
	}

	if name, ok := clean["company_name"]; ok && len([]rune(name)) < 2 {
		writeJSON(w, http.StatusBadRequest, apiError("Название компании не может быть пустым"))
		return
	}

	if err := a.store.SaveSettings(clean); err != nil {
		serverError(w, err)
		return
	}
	values, err := a.store.Settings()
	if err != nil {
		serverError(w, err)
		return
	}
	logInfo("обновлены данные компании: полей %d", len(clean))
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"settings": values,
		"message":  "Данные компании сохранены",
	}))
}
