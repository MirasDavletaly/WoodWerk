// API админ-панели. Всё, кроме входа, закрыто проверкой сессии (см. auth.go).
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxJSONBody — тело запроса от админки заведомо небольшое.
const maxJSONBody = 128 << 10

// allowedWoods — породы дерева для фильтра каталога. Пустая строка = не указана.
var allowedWoods = map[string]string{
	"oak":    "Дуб",
	"ash":    "Ясень",
	"walnut": "Орех",
	"thermo": "Термоясень",
	"wenge":  "Венге",
	"larch":  "Лиственница",
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, apiError("Нужен формат JSON"))
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректные данные формы"))
		return false
	}
	return true
}

// ------------------------------------------------------------------ вход и выход

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /api/admin/login
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	// Перебор паролей: не больше 10 попыток с адреса за 15 минут.
	if !a.logins.allow(ip, 10, 15*time.Minute) {
		writeJSON(w, http.StatusTooManyRequests,
			apiError("Слишком много попыток входа. Попробуйте через 15 минут."))
		return
	}

	var in loginInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Username = cleanLine(in.Username, 60)

	user, hash, err := a.store.UserByName(in.Username)
	if errors.Is(err, ErrNotFound) {
		// Чтобы нельзя было угадать логин по времени ответа, всё равно считаем хеш.
		verifyPassword(in.Password, "pbkdf2$sha256$210000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		writeJSON(w, http.StatusUnauthorized, apiError("Неверный логин или пароль"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if !verifyPassword(in.Password, hash) {
		writeJSON(w, http.StatusUnauthorized, apiError("Неверный логин или пароль"))
		return
	}

	token, csrf, err := a.store.CreateSession(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	a.auth.setCookie(w, r, token)
	logInfo("вход в админ-панель: %s, ip=%s", user.Username, ip)
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"user": user,
		"csrf": csrf,
	}))
}

// POST /api/admin/logout
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = a.store.DeleteSession(c.Value)
	}
	a.auth.clearCookie(w, r)
	writeJSON(w, http.StatusOK, apiOK(nil))
}

// GET /api/admin/session — кто вошёл; фронтенд берёт отсюда CSRF-токен.
func (a *API) session(w http.ResponseWriter, r *http.Request) {
	sess, user, _ := a.auth.current(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, apiError("Требуется вход в админ-панель"))
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"user":       user,
		"csrf":       sess.CSRF,
		"max_upload": a.uploads.MaxBytes(),
	}))
}

// POST /api/admin/password — смена пароля в разделе «Настройки».
type passwordInput struct {
	Current string `json:"current"`
	Next    string `json:"next"`
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request) {
	var in passwordInput
	if !decodeJSON(w, r, &in) {
		return
	}
	user := userFrom(r)
	_, hash, err := a.store.UserByID(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	if !verifyPassword(in.Current, hash) {
		writeJSON(w, http.StatusBadRequest, apiError("Текущий пароль указан неверно"))
		return
	}
	if len([]rune(in.Next)) < 8 {
		writeJSON(w, http.StatusBadRequest, apiError("Новый пароль должен быть не короче 8 символов"))
		return
	}
	if err := a.store.SetPassword(user.ID, in.Next); err != nil {
		serverError(w, err)
		return
	}
	// Остальные сессии гасим: если пароль меняли из-за утечки, чужой вход закроется.
	if err := a.store.DeleteUserSessions(user.ID, tokenFrom(r)); err != nil {
		logError(err)
	}
	logInfo("пароль администратора %s изменён", user.Username)
	writeJSON(w, http.StatusOK, apiOK(nil))
}

// ------------------------------------------------------------------ сводка

// GET /api/admin/stats
func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	st, err := a.store.Stats()
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"stats": st}))
}

// ------------------------------------------------------------------ изделия

// GET /api/admin/products
func (a *API) adminProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	catID, _ := strconv.ParseInt(q.Get("category_id"), 10, 64)

	list, err := a.store.ListProducts(ProductFilter{
		Search:     cleanLine(q.Get("search"), 80),
		CategoryID: catID,
		Status:     cleanLine(q.Get("status"), 10),
		Sort:       cleanLine(q.Get("sort"), 20),
	})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"products": list}))
}

// GET /api/admin/products/{id}
func (a *API) adminProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес изделия"))
		return
	}
	p, err := a.store.GetProduct(id, false)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Изделие не найдено"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"product": p}))
}

// validateProduct приводит данные формы в порядок и проверяет их.
// Клиентские проверки можно обойти, поэтому решает именно эта функция.
func (a *API) validateProduct(in *ProductInput) string {
	in.Title = cleanLine(in.Title, 120)
	in.TitleKK = cleanLine(in.TitleKK, 120)
	in.TitleEN = cleanLine(in.TitleEN, 120)
	in.Description = clean(in.Description, 5000)
	in.DescriptionKK = clean(in.DescriptionKK, 5000)
	in.DescriptionEN = clean(in.DescriptionEN, 5000)
	in.Badge = cleanLine(in.Badge, 24)
	in.Wood = cleanLine(in.Wood, 20)
	in.ImageURL = cleanLine(in.ImageURL, 300)

	if len([]rune(in.Title)) < 2 {
		return "Укажите название — не короче 2 символов"
	}
	if in.Price < 0 || in.Price > 1_000_000_000_000 {
		return "Проверьте цену"
	}
	if in.Status != StatusActive && in.Status != StatusHidden {
		in.Status = StatusActive
	}
	if in.Wood != "" {
		if _, ok := allowedWoods[in.Wood]; !ok {
			in.Wood = ""
		}
	}
	if in.ImageURL != "" && !isLocalPath(in.ImageURL) {
		return "Некорректный адрес фотографии"
	}

	if in.CategoryID != nil && *in.CategoryID > 0 {
		if _, err := a.store.GetCategory(*in.CategoryID); err != nil {
			return "Выбранная категория не найдена"
		}
	} else {
		in.CategoryID = nil
	}

	if len(in.Gallery) > 12 {
		return "Дополнительных фотографий может быть не больше 12"
	}
	gallery := make([]string, 0, len(in.Gallery))
	for _, u := range in.Gallery {
		u = cleanLine(u, 300)
		if u == "" {
			continue
		}
		if !isLocalPath(u) {
			return "Некорректный адрес фотографии"
		}
		gallery = append(gallery, u)
	}
	in.Gallery = gallery
	return ""
}

// isLocalPath пропускает только адреса внутри нашего сайта: так в базу
// не попадёт ни javascript:, ни чужой домен.
func isLocalPath(u string) bool {
	return strings.HasPrefix(u, "/") &&
		!strings.HasPrefix(u, "//") &&
		!strings.Contains(u, "..") &&
		!strings.ContainsAny(u, " \t\"'<>\\")
}

// POST /api/admin/products
func (a *API) createProduct(w http.ResponseWriter, r *http.Request) {
	var in ProductInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := a.validateProduct(&in); msg != "" {
		writeJSON(w, http.StatusBadRequest, apiError(msg))
		return
	}
	p, err := a.store.CreateProduct(in)
	if err != nil {
		serverError(w, err)
		return
	}
	logInfo("добавлено изделие #%d «%s»", p.ID, p.Title)
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"product": p,
		"message": "Мебель успешно добавлена",
	}))
}

// PUT /api/admin/products/{id}
func (a *API) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес изделия"))
		return
	}
	var in ProductInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := a.validateProduct(&in); msg != "" {
		writeJSON(w, http.StatusBadRequest, apiError(msg))
		return
	}

	before, err := a.store.GetProduct(id, false)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Изделие не найдено"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	p, err := a.store.UpdateProduct(id, in)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Изделие не найдено"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	// Картинки, которые перестали использоваться, убираем с диска.
	a.dropUnusedImages(before, p)

	logInfo("изменено изделие #%d «%s»", p.ID, p.Title)
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"product": p,
		"message": "Изменения успешно сохранены",
	}))
}

// PATCH /api/admin/products/{id}/status — «Скрыть» / «Показать».
func (a *API) setProductStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес изделия"))
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Status != StatusActive && in.Status != StatusHidden {
		writeJSON(w, http.StatusBadRequest, apiError("Неизвестный статус"))
		return
	}

	p, err := a.store.SetProductStatus(id, in.Status)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Изделие не найдено"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	message := "Изделие показано на сайте"
	if in.Status == StatusHidden {
		message = "Изделие скрыто с сайта"
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"product": p, "message": message}))
}

// DELETE /api/admin/products/{id}
func (a *API) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес изделия"))
		return
	}
	urls, err := a.store.DeleteProduct(id)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Изделие не найдено"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	for _, u := range urls {
		a.removeIfUnused(u)
	}
	logInfo("удалено изделие #%d", id)
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"message": "Мебель успешно удалена"}))
}

// dropUnusedImages сравнивает старый и новый набор картинок изделия
// и удаляет с диска то, что больше нигде не встречается.
func (a *API) dropUnusedImages(before, after *Product) {
	keep := map[string]bool{after.ImageURL: true}
	for _, im := range after.Images {
		keep[im.ImageURL] = true
	}

	old := []string{before.ImageURL}
	for _, im := range before.Images {
		old = append(old, im.ImageURL)
	}
	for _, u := range old {
		if u != "" && !keep[u] {
			a.removeIfUnused(u)
		}
	}
}

func (a *API) removeIfUnused(url string) {
	if url == "" {
		return
	}
	used, err := a.store.UsedImage(url)
	if err != nil {
		logError(err)
		return
	}
	if used {
		return
	}
	if err := a.uploads.Delete(url); err != nil {
		logError(err)
	}
}

// ------------------------------------------------------------------ категории

// GET /api/admin/categories
func (a *API) adminCategories(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.ListCategories()
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"categories": list}))
}

type categoryInput struct {
	Name   string `json:"name"`
	NameKK string `json:"name_kk"`
	NameEN string `json:"name_en"`
}

func validCategoryName(name string) (string, string) {
	name = cleanLine(name, 60)
	if len([]rune(name)) < 2 {
		return "", "Название категории должно быть не короче 2 символов"
	}
	return name, ""
}

// POST /api/admin/categories
func (a *API) createCategory(w http.ResponseWriter, r *http.Request) {
	var in categoryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	name, msg := validCategoryName(in.Name)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, apiError(msg))
		return
	}
	c, err := a.store.CreateCategoryTr(name,
		cleanLine(in.NameKK, 60), cleanLine(in.NameEN, 60))
	if errors.Is(err, ErrDuplicate) {
		writeJSON(w, http.StatusBadRequest, apiError("Такая категория уже есть"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"category": c,
		"message":  "Категория успешно добавлена",
	}))
}

// PUT /api/admin/categories/{id}
func (a *API) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес категории"))
		return
	}
	var in categoryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	name, msg := validCategoryName(in.Name)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, apiError(msg))
		return
	}
	c, err := a.store.UpdateCategory(id, name,
		cleanLine(in.NameKK, 60), cleanLine(in.NameEN, 60))
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Категория не найдена"))
		return
	}
	if errors.Is(err, ErrDuplicate) {
		writeJSON(w, http.StatusBadRequest, apiError("Такая категория уже есть"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"category": c,
		"message":  "Изменения успешно сохранены",
	}))
}

// DELETE /api/admin/categories/{id}
func (a *API) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес категории"))
		return
	}
	err := a.store.DeleteCategory(id)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Категория не найдена"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"message": "Категория успешно удалена"}))
}

// ------------------------------------------------------------------ фотографии

// POST /api/admin/upload — принимает одно или несколько изображений.
func (a *API) upload(w http.ResponseWriter, r *http.Request) {
	limit := a.uploads.MaxBytes()*12 + (1 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, limit)

	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest,
			apiError("Не удалось прочитать файл. Возможно, он больше "+humanSize(a.uploads.MaxBytes())+"."))
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError("Файл не выбран"))
		return
	}
	if len(files) > 12 {
		writeJSON(w, http.StatusBadRequest, apiError("За один раз можно загрузить не больше 12 фотографий"))
		return
	}

	urls := make([]string, 0, len(files))
	for _, fh := range files {
		url, err := a.uploads.Save(fh)
		if errors.Is(err, ErrTooBig) {
			writeJSON(w, http.StatusBadRequest,
				apiError("Файл «"+cleanLine(fh.Filename, 60)+"» больше "+humanSize(a.uploads.MaxBytes())))
			return
		}
		if errors.Is(err, ErrBadImage) {
			writeJSON(w, http.StatusBadRequest,
				apiError("Файл «"+cleanLine(fh.Filename, 60)+"»: "+ErrBadImage.Error()))
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		urls = append(urls, url)
	}

	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"urls":    urls,
		"url":     urls[0],
		"message": "Фотография загружена",
	}))
}

// POST /api/admin/upload/delete — убрать неиспользуемый файл с диска.
func (a *API) deleteUpload(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	a.removeIfUnused(cleanLine(in.URL, 300))
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"message": "Фотография удалена"}))
}
