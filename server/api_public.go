// Публичное API сайта: только чтение и только то, что администратор
// пометил как «Активен».
package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

// API — общий контекст обработчиков.
type API struct {
	store   *Store
	uploads *Uploads
	auth    *Auth
	logins  *limiter // защита формы входа от перебора паролей
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// serverError прячет подробности от посетителя, но пишет их в лог.
func serverError(w http.ResponseWriter, err error) {
	logError(err)
	writeJSON(w, http.StatusInternalServerError, apiError("Произошла ошибка. Попробуйте ещё раз."))
}

// langOf читает язык из запроса. Неизвестное значение — русский:
// подставлять пустоту или падать из-за параметра в адресе нельзя.
func langOf(r *http.Request) string {
	switch r.URL.Query().Get("lang") {
	case "kk":
		return "kk"
	case "en":
		return "en"
	default:
		return "ru"
	}
}

// GET /api/products — каталог для публичного сайта.
func (a *API) publicProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := a.store.ListProducts(ProductFilter{
		OnlyActive:   true,
		CategorySlug: cleanLine(q.Get("category"), 60),
		Wood:         cleanLine(q.Get("wood"), 40),
		Search:       cleanLine(q.Get("search"), 80),
		Sort:         cleanLine(q.Get("sort"), 20),
	})
	if err != nil {
		serverError(w, err)
		return
	}

	lang := langOf(r)
	for i := range list {
		list[i].Localize(lang)
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"products": list}))
}

// GET /api/products/{id} — карточка изделия.
func (a *API) publicProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес изделия"))
		return
	}
	p, err := a.store.GetProduct(id, true)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Изделие не найдено"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	p.Localize(langOf(r))
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"product": p}))
}

// GET /api/categories — разделы каталога вместе с числом изделий.
func (a *API) publicCategories(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.ListCategories()
	if err != nil {
		serverError(w, err)
		return
	}

	lang := langOf(r)
	for i := range list {
		list[i].Localize(lang)
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"categories": list}))
}
