// Галерея «Панели в интерьере» — редактируемый из админки блок главной.
//
// Раньше карточки лежали прямо в index.html вместе с ключами перевода,
// и менять их можно было только правкой файла. Здесь они живут в базе:
// модель, запросы, первичное наполнение и обработчики собраны в одном
// файле, чтобы по этому же образцу добавить остальные блоки главной.
package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

// ------------------------------------------------------------------ модель

// GalleryItem — одна карточка галереи: снимок интерьера и подпись к нему.
type GalleryItem struct {
	ID        int64  `json:"id"`
	ImageURL  string `json:"image_url"`
	Alt       string `json:"alt"`
	Title     string `json:"title"`
	TitleKK   string `json:"title_kk"`
	TitleEN   string `json:"title_en"`
	Caption   string `json:"caption"`
	CaptionKK string `json:"caption_kk"`
	CaptionEN string `json:"caption_en"`
	SortOrder int    `json:"sort_order"`
	Visible   bool   `json:"visible"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Localize подставляет перевод там, где он заполнен. Пустой перевод значит
// «показывать русский» — то же правило, что у изделий и разделов.
func (g *GalleryItem) Localize(lang string) {
	switch lang {
	case "kk":
		g.Title = pick(g.TitleKK, g.Title)
		g.Caption = pick(g.CaptionKK, g.Caption)
	case "en":
		g.Title = pick(g.TitleEN, g.Title)
		g.Caption = pick(g.CaptionEN, g.Caption)
	}
}

// GalleryInput — то, что приходит из формы админки.
type GalleryInput struct {
	ImageURL  string `json:"image_url"`
	Alt       string `json:"alt"`
	Title     string `json:"title"`
	TitleKK   string `json:"title_kk"`
	TitleEN   string `json:"title_en"`
	Caption   string `json:"caption"`
	CaptionKK string `json:"caption_kk"`
	CaptionEN string `json:"caption_en"`
	Visible   bool   `json:"visible"`
}

// ------------------------------------------------------------------ запросы

const galleryColumns = `id, image_url, alt, title, title_kk, title_en,
        caption, caption_kk, caption_en, sort_order, visible, created_at, updated_at`

func scanGallery(rows *sql.Rows) (GalleryItem, error) {
	var g GalleryItem
	err := rows.Scan(&g.ID, &g.ImageURL, &g.Alt, &g.Title, &g.TitleKK, &g.TitleEN,
		&g.Caption, &g.CaptionKK, &g.CaptionEN, &g.SortOrder, &g.Visible,
		&g.CreatedAt, &g.UpdatedAt)
	return g, err
}

// ListGallery отдаёт карточки в заданном администратором порядке.
// onlyVisible включён для сайта и выключен для админки.
func (s *Store) ListGallery(onlyVisible bool) ([]GalleryItem, error) {
	q := `SELECT ` + galleryColumns + ` FROM gallery`
	if onlyVisible {
		q += ` WHERE visible = 1`
	}
	q += ` ORDER BY sort_order, id`

	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]GalleryItem, 0, 12)
	for rows.Next() {
		g, err := scanGallery(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

func (s *Store) GetGalleryItem(id int64) (*GalleryItem, error) {
	rows, err := s.db.Query(`SELECT `+galleryColumns+` FROM gallery WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	g, err := scanGallery(rows)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// nextGallerySort ставит новую карточку в конец списка.
func (s *Store) nextGallerySort() (int, error) {
	var max sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(sort_order) FROM gallery`).Scan(&max); err != nil {
		return 0, err
	}
	return int(max.Int64) + 10, nil
}

func (s *Store) CreateGalleryItem(in GalleryInput) (*GalleryItem, error) {
	order, err := s.nextGallerySort()
	if err != nil {
		return nil, err
	}
	ts := now()
	res, err := s.db.Exec(`
        INSERT INTO gallery (image_url, alt, title, title_kk, title_en,
                             caption, caption_kk, caption_en,
                             sort_order, visible, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ImageURL, in.Alt, in.Title, in.TitleKK, in.TitleEN,
		in.Caption, in.CaptionKK, in.CaptionEN, order, in.Visible, ts, ts)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetGalleryItem(id)
}

func (s *Store) UpdateGalleryItem(id int64, in GalleryInput) (*GalleryItem, error) {
	res, err := s.db.Exec(`
        UPDATE gallery
           SET image_url = ?, alt = ?, title = ?, title_kk = ?, title_en = ?,
               caption = ?, caption_kk = ?, caption_en = ?, visible = ?, updated_at = ?
         WHERE id = ?`,
		in.ImageURL, in.Alt, in.Title, in.TitleKK, in.TitleEN,
		in.Caption, in.CaptionKK, in.CaptionEN, in.Visible, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetGalleryItem(id)
}

// DeleteGalleryItem возвращает адрес снятого снимка: вызывающая сторона
// решает, удалять ли сам файл (он может использоваться и в другом месте).
func (s *Store) DeleteGalleryItem(id int64) (string, error) {
	item, err := s.GetGalleryItem(id)
	if err != nil {
		return "", err
	}
	if _, err := s.db.Exec(`DELETE FROM gallery WHERE id = ?`, id); err != nil {
		return "", err
	}
	return item.ImageURL, nil
}

// ReorderGallery раскладывает карточки в присланном порядке. Идентификаторы,
// которых нет в списке, сохраняют свои места в хвосте.
func (s *Store) ReorderGallery(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE gallery SET sort_order = ?, updated_at = ? WHERE id = ?`,
			(i+1)*10, now(), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ------------------------------------------------------------------ обработчики

// GET /api/gallery — карточки для главной страницы сайта.
func (a *API) publicGallery(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.ListGallery(true)
	if err != nil {
		serverError(w, err)
		return
	}
	lang := langOf(r)
	for i := range list {
		list[i].Localize(lang)
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"gallery": list}))
}

// GET /api/admin/gallery
func (a *API) adminGallery(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.ListGallery(false)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"gallery": list}))
}

// cleanGalleryInput приводит присланные поля к допустимым значениям и
// возвращает текст ошибки, если карточку принимать нельзя.
func cleanGalleryInput(in GalleryInput) (GalleryInput, string) {
	out := GalleryInput{
		ImageURL:  cleanLine(in.ImageURL, 300),
		Alt:       cleanLine(in.Alt, 160),
		Title:     cleanLine(in.Title, 80),
		TitleKK:   cleanLine(in.TitleKK, 80),
		TitleEN:   cleanLine(in.TitleEN, 80),
		Caption:   cleanLine(in.Caption, 200),
		CaptionKK: cleanLine(in.CaptionKK, 200),
		CaptionEN: cleanLine(in.CaptionEN, 200),
		Visible:   in.Visible,
	}
	if len([]rune(out.Title)) < 2 {
		return out, "Название карточки должно быть не короче 2 символов"
	}
	// Принимаем только свои файлы: чужой адрес в src открыл бы страницу
	// для подгрузки картинки со стороннего сервера.
	if !strings.HasPrefix(out.ImageURL, "/uploads/") &&
		!strings.HasPrefix(out.ImageURL, "/assets/") {
		return out, "Выберите изображение из загруженных на сайт"
	}
	// Подпись для читалки экрана: если её не заполнили, берём название.
	if out.Alt == "" {
		out.Alt = out.Title
	}
	return out, ""
}

// POST /api/admin/gallery
func (a *API) createGalleryItem(w http.ResponseWriter, r *http.Request) {
	var in GalleryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	clean, msg := cleanGalleryInput(in)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, apiError(msg))
		return
	}
	item, err := a.store.CreateGalleryItem(clean)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"item":    item,
		"message": "Карточка добавлена в галерею",
	}))
}

// PUT /api/admin/gallery/{id}
func (a *API) updateGalleryItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес карточки"))
		return
	}
	var in GalleryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	clean, msg := cleanGalleryInput(in)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, apiError(msg))
		return
	}
	item, err := a.store.UpdateGalleryItem(id, clean)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Карточка не найдена"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{
		"item":    item,
		"message": "Изменения сохранены",
	}))
}

// DELETE /api/admin/gallery/{id}
func (a *API) deleteGalleryItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError("Некорректный адрес карточки"))
		return
	}
	url, err := a.store.DeleteGalleryItem(id)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiError("Карточка не найдена"))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	// Файл убираем только если на него больше никто не ссылается —
	// ни каталог, ни оставшиеся карточки галереи. Снимки из /assets/
	// Uploads.Delete не трогает: он работает только внутри /uploads/.
	a.removeIfUnused(url)
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"message": "Карточка удалена"}))
}

// POST /api/admin/gallery/reorder
func (a *API) reorderGallery(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError("Пустой список карточек"))
		return
	}
	if err := a.store.ReorderGallery(in.IDs); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiOK(map[string]any{"message": "Порядок сохранён"}))
}

// ------------------------------------------------------------------ наполнение

// seedGalleryItem — карточка из поставки. Тексты повторяют те, что были
// свёрстаны в index.html, чтобы после обновления главная выглядела так же.
type seedGalleryItem struct {
	Image     string
	Alt       string
	Title     string
	TitleKK   string
	TitleEN   string
	Caption   string
	CaptionKK string
	CaptionEN string
}

var seedGallery = []seedGalleryItem{
	{
		Image: "/assets/img/scenes/kitchen.svg",
		Alt:   "Кухня-гостиная с панелями на стене",
		Title: "Кухня-гостиная", TitleKK: "Ас үй-қонақ бөлме", TitleEN: "Kitchen-living room",
		Caption:   "Стена у обеденной зоны — декор под дерево",
		CaptionKK: "Ас ішетін аймақтағы қабырға — ағаш өрнегі",
		CaptionEN: "The wall by the dining area — a wood-look decor",
	},
	{
		Image: "/assets/img/scenes/wardrobe.svg",
		Alt:   "Гардеробная с матовыми панелями",
		Title: "Гардеробная", TitleKK: "Киім бөлмесі", TitleEN: "Wardrobe room",
		Caption:   "Ниши и торцы — матовые панели без бликов",
		CaptionKK: "Қуыстар мен шеттер — жарқырамайтын күңгірт панельдер",
		CaptionEN: "Niches and edges — matte panels without glare",
	},
	{
		Image: "/assets/img/scenes/stairs.svg",
		Alt:   "Холл с лестницей и панелями под камень",
		Title: "Холл с лестницей", TitleKK: "Баспалдағы бар холл", TitleEN: "Hall with a staircase",
		Caption:   "Стена вдоль марша — панель под камень",
		CaptionKK: "Баспалдақ бойындағы қабырға — тас өрнегі",
		CaptionEN: "The wall along the flight — a stone-look panel",
	},
	{
		Image: "/assets/img/scenes/panels.svg",
		Alt:   "Прихожая с декоративными рейками",
		Title: "Прихожая", TitleKK: "Кіреберіс", TitleEN: "Entrance hall",
		Caption:   "Акцентная стена — декоративные рейки",
		CaptionKK: "Екпінді қабырға — сәндік рейкалар",
		CaptionEN: "An accent wall — decorative slats",
	},
	{
		Image: "/assets/img/scenes/bed.svg",
		Alt:   "Спальня с панелями за изголовьем",
		Title: "Спальня", TitleKK: "Жатын бөлме", TitleEN: "Bedroom",
		Caption:   "Изголовье во всю стену — панель под ткань",
		CaptionKK: "Қабырғаның бүкіл ұзындығындағы бас жағы — мата өрнегі",
		CaptionEN: "A headboard across the wall — a fabric-look panel",
	},
	{
		Image: "/assets/img/scenes/office.svg",
		Alt:   "Кабинет с панелями под дерево",
		Title: "Кабинет", TitleKK: "Жұмыс бөлмесі", TitleEN: "Study",
		Caption:   "Стена за столом — тёплый древесный декор",
		CaptionKK: "Үстел артындағы қабырға — жылы ағаш өрнегі",
		CaptionEN: "The wall behind the desk — a warm wood decor",
	},
}

// SeedGallery наполняет галерею только на пустой таблице: правки
// администратора повторный запуск сервера не затирает.
func (s *Store) SeedGallery() error {
	list, err := s.ListGallery(false)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return nil
	}
	for _, it := range seedGallery {
		if _, err := s.CreateGalleryItem(GalleryInput{
			ImageURL:  it.Image,
			Alt:       it.Alt,
			Title:     it.Title,
			TitleKK:   it.TitleKK,
			TitleEN:   it.TitleEN,
			Caption:   it.Caption,
			CaptionKK: it.CaptionKK,
			CaptionEN: it.CaptionEN,
			Visible:   true,
		}); err != nil {
			return err
		}
	}
	return nil
}
