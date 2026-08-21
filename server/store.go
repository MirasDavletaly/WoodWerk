package main

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ------------------------------------------------------------------ модели

// Category — раздел каталога («Диваны», «Кровати» и т. д.).
type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	Products  int    `json:"products"` // сколько изделий в разделе
}

// ProductImage — дополнительная фотография изделия.
type ProductImage struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"product_id"`
	ImageURL  string `json:"image_url"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
}

// Product — мебельное изделие.
type Product struct {
	ID           int64          `json:"id"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Price        int64          `json:"price"`
	ImageURL     string         `json:"image_url"`
	CategoryID   *int64         `json:"category_id"`
	CategoryName string         `json:"category_name"`
	CategorySlug string         `json:"category_slug"`
	Status       string         `json:"status"` // active | hidden
	Wood         string         `json:"wood"`
	Badge        string         `json:"badge"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
	Images       []ProductImage `json:"images"`
}

// Stats — цифры для главной страницы админки.
type Stats struct {
	Total      int       `json:"total"`
	Active     int       `json:"active"`
	Hidden     int       `json:"hidden"`
	Categories int       `json:"categories"`
	Recent     []Product `json:"recent"`
}

// ProductFilter описывает выборку списка изделий.
type ProductFilter struct {
	Search       string
	CategoryID   int64  // 0 = любая
	CategorySlug string // альтернатива CategoryID, используется публичным API
	Status       string // active | hidden | "" = любой
	Wood         string
	Sort         string // new | old | price-asc | price-desc | name
	OnlyActive   bool   // публичный каталог не видит скрытые
}

const (
	StatusActive = "active"
	StatusHidden = "hidden"
)

// ErrNotFound возвращается, когда записи с таким id нет.
var ErrNotFound = errors.New("запись не найдена")

// ErrDuplicate — нарушение уникальности (например, категория с таким именем).
var ErrDuplicate = errors.New("такая запись уже существует")

// Store — все запросы к базе в одном месте.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// now отдаёт время в UTC и RFC3339: такие строки корректно сортируются как текст.
func now() string { return time.Now().UTC().Format(time.RFC3339) }

// searchText готовит строку, по которой ищет админка: регистр русских букв
// SQLite привести не умеет, поэтому делаем это заранее и в Go.
func searchText(title, description string) string {
	return strings.ToLower(title + " " + description)
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

// ------------------------------------------------------------------ категории

func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.db.Query(`
        SELECT c.id, c.name, c.slug, c.sort_order, c.created_at,
               (SELECT COUNT(*) FROM products p WHERE p.category_id = c.id)
        FROM categories c
        ORDER BY c.sort_order, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.SortOrder, &c.CreatedAt, &c.Products); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) GetCategory(id int64) (*Category, error) {
	var c Category
	err := s.db.QueryRow(`
        SELECT id, name, slug, sort_order, created_at
        FROM categories WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Slug, &c.SortOrder, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Store) CreateCategory(name string) (*Category, error) {
	slug, err := s.freeSlug(slugify(name), 0)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(
		`INSERT INTO categories (name, slug, sort_order, created_at) VALUES (?, ?, ?, ?)`,
		name, slug, 100, now())
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
	return s.GetCategory(id)
}

func (s *Store) UpdateCategory(id int64, name string) (*Category, error) {
	if _, err := s.GetCategory(id); err != nil {
		return nil, err
	}
	slug, err := s.freeSlug(slugify(name), id)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`UPDATE categories SET name = ?, slug = ? WHERE id = ?`, name, slug, id)
	if isUnique(err) {
		return nil, ErrDuplicate
	}
	if err != nil {
		return nil, err
	}
	return s.GetCategory(id)
}

// DeleteCategory удаляет раздел. Изделия не пропадают: у них просто
// обнуляется категория (ON DELETE SET NULL).
func (s *Store) DeleteCategory(id int64) error {
	res, err := s.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// freeSlug подбирает свободный слаг: base, base-2, base-3 …
func (s *Store) freeSlug(base string, exceptID int64) (string, error) {
	if base == "" {
		base = "razdel"
	}
	candidate := base
	for i := 2; i < 200; i++ {
		var id int64
		err := s.db.QueryRow(`SELECT id FROM categories WHERE slug = ?`, candidate).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && id == exceptID) {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		candidate = base + "-" + strconv.Itoa(i)
	}
	return base + "-" + strconv.FormatInt(time.Now().Unix()%100000, 10), nil
}

// ------------------------------------------------------------------ изделия

const productColumns = `
    p.id, p.title, p.description, p.price, p.image_url, p.category_id,
    COALESCE(c.name, ''), COALESCE(c.slug, ''),
    p.status, p.wood, p.badge, p.created_at, p.updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanProduct(sc scanner) (Product, error) {
	var p Product
	var catID sql.NullInt64
	err := sc.Scan(&p.ID, &p.Title, &p.Description, &p.Price, &p.ImageURL, &catID,
		&p.CategoryName, &p.CategorySlug, &p.Status, &p.Wood, &p.Badge,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	if catID.Valid {
		id := catID.Int64
		p.CategoryID = &id
	}
	p.Images = []ProductImage{}
	return p, nil
}

func (s *Store) ListProducts(f ProductFilter) ([]Product, error) {
	q := `SELECT ` + productColumns + `
          FROM products p LEFT JOIN categories c ON c.id = p.category_id
          WHERE 1 = 1`
	args := []any{}

	if f.OnlyActive {
		q += ` AND p.status = ?`
		args = append(args, StatusActive)
	} else if f.Status == StatusActive || f.Status == StatusHidden {
		q += ` AND p.status = ?`
		args = append(args, f.Status)
	}
	if f.CategoryID > 0 {
		q += ` AND p.category_id = ?`
		args = append(args, f.CategoryID)
	}
	if f.CategorySlug != "" {
		q += ` AND c.slug = ?`
		args = append(args, f.CategorySlug)
	}
	if f.Wood != "" {
		q += ` AND p.wood = ?`
		args = append(args, f.Wood)
	}
	if f.Search != "" {
		// Регистр приводим здесь: LOWER() в SQLite не знает про кириллицу,
		// поэтому в базе лежит уже готовая строка для поиска.
		// Символы % и _ внутри запроса пользователя — обычный текст, а не шаблон.
		esc := strings.NewReplacer("#", "##", "%", "#%", "_", "#_").Replace(strings.ToLower(f.Search))
		q += ` AND p.search_text LIKE ? ESCAPE '#'`
		args = append(args, "%"+esc+"%")
	}

	switch f.Sort {
	case "price-asc":
		q += ` ORDER BY p.price ASC, p.id DESC`
	case "price-desc":
		q += ` ORDER BY p.price DESC, p.id DESC`
	case "name":
		q += ` ORDER BY p.title COLLATE NOCASE ASC`
	case "old":
		q += ` ORDER BY p.created_at ASC, p.id ASC`
	default:
		q += ` ORDER BY p.created_at DESC, p.id DESC`
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// GetProduct отдаёт изделие вместе с дополнительными фотографиями.
func (s *Store) GetProduct(id int64, onlyActive bool) (*Product, error) {
	q := `SELECT ` + productColumns + `
          FROM products p LEFT JOIN categories c ON c.id = p.category_id
          WHERE p.id = ?`
	args := []any{id}
	if onlyActive {
		q += ` AND p.status = ?`
		args = append(args, StatusActive)
	}

	p, err := scanProduct(s.db.QueryRow(q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	images, err := s.productImages(id)
	if err != nil {
		return nil, err
	}
	p.Images = images
	return &p, nil
}

func (s *Store) productImages(productID int64) ([]ProductImage, error) {
	rows, err := s.db.Query(`
        SELECT id, product_id, image_url, sort_order, created_at
        FROM product_images WHERE product_id = ?
        ORDER BY sort_order, id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []ProductImage{}
	for rows.Next() {
		var im ProductImage
		if err := rows.Scan(&im.ID, &im.ProductID, &im.ImageURL, &im.SortOrder, &im.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, im)
	}
	return list, rows.Err()
}

// ProductInput — то, что приходит из формы админки.
type ProductInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	ImageURL    string   `json:"image_url"`
	CategoryID  *int64   `json:"category_id"`
	Status      string   `json:"status"`
	Wood        string   `json:"wood"`
	Badge       string   `json:"badge"`
	Gallery     []string `json:"gallery"` // дополнительные фотографии
}

func (s *Store) CreateProduct(in ProductInput) (*Product, error) {
	ts := now()
	res, err := s.db.Exec(`
        INSERT INTO products (title, description, price, image_url, category_id,
                              status, wood, badge, search_text, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Title, in.Description, in.Price, in.ImageURL, catArg(in.CategoryID),
		in.Status, in.Wood, in.Badge, searchText(in.Title, in.Description), ts, ts)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if err := s.setGallery(id, in.Gallery); err != nil {
		return nil, err
	}
	return s.GetProduct(id, false)
}

func (s *Store) UpdateProduct(id int64, in ProductInput) (*Product, error) {
	res, err := s.db.Exec(`
        UPDATE products
        SET title = ?, description = ?, price = ?, image_url = ?, category_id = ?,
            status = ?, wood = ?, badge = ?, search_text = ?, updated_at = ?
        WHERE id = ?`,
		in.Title, in.Description, in.Price, in.ImageURL, catArg(in.CategoryID),
		in.Status, in.Wood, in.Badge, searchText(in.Title, in.Description), now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	if err := s.setGallery(id, in.Gallery); err != nil {
		return nil, err
	}
	return s.GetProduct(id, false)
}

// DeleteProduct удаляет изделие и возвращает адреса его картинок,
// чтобы вызывающий код мог убрать файлы с диска.
func (s *Store) DeleteProduct(id int64) ([]string, error) {
	p, err := s.GetProduct(id, false)
	if err != nil {
		return nil, err
	}
	urls := []string{p.ImageURL}
	for _, im := range p.Images {
		urls = append(urls, im.ImageURL)
	}
	if _, err := s.db.Exec(`DELETE FROM products WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return urls, nil
}

func (s *Store) SetProductStatus(id int64, status string) (*Product, error) {
	res, err := s.db.Exec(`UPDATE products SET status = ?, updated_at = ? WHERE id = ?`,
		status, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetProduct(id, false)
}

// setGallery полностью переписывает список дополнительных фотографий.
func (s *Store) setGallery(productID int64, urls []string) error {
	if _, err := s.db.Exec(`DELETE FROM product_images WHERE product_id = ?`, productID); err != nil {
		return err
	}
	ts := now()
	for i, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, err := s.db.Exec(`
            INSERT INTO product_images (product_id, image_url, sort_order, created_at)
            VALUES (?, ?, ?, ?)`, productID, u, i, ts); err != nil {
			return err
		}
	}
	return nil
}

func catArg(id *int64) any {
	if id == nil || *id <= 0 {
		return nil
	}
	return *id
}

// UsedImage сообщает, используется ли файл хоть где-то: перед удалением
// с диска надо убедиться, что на него больше никто не ссылается.
func (s *Store) UsedImage(url string) (bool, error) {
	var n int
	err := s.db.QueryRow(`
        SELECT (SELECT COUNT(*) FROM products WHERE image_url = ?) +
               (SELECT COUNT(*) FROM product_images WHERE image_url = ?)`,
		url, url).Scan(&n)
	return n > 0, err
}

// ------------------------------------------------------------------ сводка

func (s *Store) Stats() (*Stats, error) {
	var st Stats
	err := s.db.QueryRow(`
        SELECT (SELECT COUNT(*) FROM products),
               (SELECT COUNT(*) FROM products WHERE status = 'active'),
               (SELECT COUNT(*) FROM products WHERE status = 'hidden'),
               (SELECT COUNT(*) FROM categories)`).
		Scan(&st.Total, &st.Active, &st.Hidden, &st.Categories)
	if err != nil {
		return nil, err
	}

	recent, err := s.ListProducts(ProductFilter{Sort: "new"})
	if err != nil {
		return nil, err
	}
	if len(recent) > 5 {
		recent = recent[:5]
	}
	st.Recent = recent
	return &st, nil
}
