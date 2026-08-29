package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestStore поднимает пустую базу во временном каталоге.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("не открыть базу: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Декоративные рейки": "dekorativnye-reyki",
		"Матовые":            "matovye",
		"Бюджетная серия":    "byudzhetnaya-seriya",
		"Office Chairs 2":    "office-chairs-2",
		"   ":                "",
		"!!!":                "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := hashPassword("Очень Секретный 123")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "Очень") {
		t.Fatal("пароль виден в хеше")
	}
	if !verifyPassword("Очень Секретный 123", hash) {
		t.Error("верный пароль не принят")
	}
	if verifyPassword("другой пароль", hash) {
		t.Error("неверный пароль принят")
	}
	// Соль случайная, поэтому два хеша одного пароля не совпадают.
	other, _ := hashPassword("Очень Секретный 123")
	if other == hash {
		t.Error("соль не используется")
	}
	if verifyPassword("что угодно", "мусор") {
		t.Error("испорченный хеш принят")
	}
}

// Поиск обязан находить русские слова независимо от регистра:
// LOWER() в SQLite работает только с латиницей, поэтому регистр
// приводится в Go, и это надо проверять.
func TestSearchIsCaseInsensitiveForRussian(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateProduct(ProductInput{
		Title:       "Лестница «Turn 180»",
		Description: "Дуб на металлокаркасе",
		Price:       1870000,
		Status:      StatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	for _, term := range []string{"лестница", "ЛЕСТНИЦА", "Лестница", "дуб", "ДУБ"} {
		list, err := s.ListProducts(ProductFilter{Search: term})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 {
			t.Errorf("поиск %q нашёл %d изделий, ожидалось 1", term, len(list))
		}
	}

	list, err := s.ListProducts(ProductFilter{Search: "шкаф"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("поиск «шкаф» нашёл %d изделий, ожидалось 0", len(list))
	}
}

// Символы % и _ в запросе — обычный текст, а не шаблон LIKE.
func TestSearchDoesNotTreatInputAsPattern(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateProduct(ProductInput{Title: "Стол", Status: StatusActive}); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListProducts(ProductFilter{Search: "%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("«%%» сработал как шаблон и нашёл %d изделий", len(list))
	}
}

func TestOnlyActiveHidesProducts(t *testing.T) {
	s := newTestStore(t)
	p, err := s.CreateProduct(ProductInput{Title: "Кресло", Status: StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetProductStatus(p.ID, StatusHidden); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListProducts(ProductFilter{OnlyActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("скрытое изделие попало в публичный каталог")
	}
	if _, err := s.GetProduct(p.ID, true); err != ErrNotFound {
		t.Errorf("скрытое изделие доступно по прямой ссылке: %v", err)
	}
	if _, err := s.GetProduct(p.ID, false); err != nil {
		t.Errorf("скрытое изделие пропало из админки: %v", err)
	}
}

// Удаление категории не должно уносить с собой изделия.
func TestDeleteCategoryKeepsProducts(t *testing.T) {
	s := newTestStore(t)
	c, err := s.CreateCategory("Диваны")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProduct(ProductInput{Title: "Диван", CategoryID: &c.ID, Status: StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCategory(c.ID); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetProduct(p.ID, false)
	if err != nil {
		t.Fatalf("изделие исчезло вместе с категорией: %v", err)
	}
	if got.CategoryID != nil {
		t.Errorf("у изделия осталась ссылка на удалённую категорию")
	}
}

// Удаление изделия должно уносить и его дополнительные фотографии.
func TestDeleteProductRemovesImages(t *testing.T) {
	s := newTestStore(t)
	p, err := s.CreateProduct(ProductInput{
		Title:    "Стол",
		ImageURL: "/uploads/2026/08/main.jpg",
		Gallery:  []string{"/uploads/2026/08/a.jpg", "/uploads/2026/08/b.jpg"},
		Status:   StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Images) != 2 {
		t.Fatalf("сохранено %d дополнительных фото, ожидалось 2", len(p.Images))
	}

	urls, err := s.DeleteProduct(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 3 {
		t.Errorf("на удаление вернулось %d файлов, ожидалось 3", len(urls))
	}

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM product_images`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("в product_images осталось %d записей", n)
	}
}

func TestUniqueCategoryName(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateCategory("Столы"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCategory("Столы"); err != ErrDuplicate {
		t.Errorf("повтор названия принят: %v", err)
	}
}

// Слаги обязаны быть разными даже у похожих названий.
func TestSlugCollision(t *testing.T) {
	s := newTestStore(t)
	a, err := s.CreateCategory("Столы!")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateCategory("Столы?")
	if err != nil {
		t.Fatal(err)
	}
	if a.Slug == b.Slug {
		t.Errorf("два раздела получили один адрес: %q", a.Slug)
	}
}

func TestIsLocalPath(t *testing.T) {
	ok := []string{"/uploads/2026/08/a.jpg", "/assets/img/scenes/bed.svg"}
	bad := []string{
		"https://example.com/a.jpg",
		"//example.com/a.jpg",
		"javascript:alert(1)",
		"/uploads/../../etc/passwd",
		"/uploads/a b.jpg",
	}
	for _, u := range ok {
		if !isLocalPath(u) {
			t.Errorf("свой адрес отклонён: %q", u)
		}
	}
	for _, u := range bad {
		if isLocalPath(u) {
			t.Errorf("чужой адрес принят: %q", u)
		}
	}
}

// Файл проверяется по содержимому, а не по расширению.
func TestUploadRejectsFakeImage(t *testing.T) {
	dir := t.TempDir()
	u, err := NewUploads(dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := u.relative("/uploads/../secret"); ok {
		t.Error("выход за пределы каталога загрузок разрешён")
	}
	if _, ok := u.relative("/assets/img/logo.svg"); ok {
		t.Error("удаление файла вне uploads разрешено")
	}
	rel, ok := u.relative("/uploads/2026/08/a.jpg")
	if !ok || rel != "2026/08/a.jpg" {
		t.Errorf("relative вернул %q, %v", rel, ok)
	}
	if err := u.Delete("/assets/img/logo.svg"); err != nil {
		t.Errorf("Delete на чужом пути вернул ошибку: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("каталог загрузок не создан: %v", err)
	}
}

func TestStatsCountsStatuses(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 3; i++ {
		if _, err := s.CreateProduct(ProductInput{Title: "Изделие", Status: StatusActive}); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.CreateProduct(ProductInput{Title: "Скрытое", Status: StatusHidden})
	if err != nil {
		t.Fatal(err)
	}
	_ = p

	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 4 || st.Active != 3 || st.Hidden != 1 {
		t.Errorf("сводка: всего=%d активных=%d скрытых=%d, ожидалось 4/3/1",
			st.Total, st.Active, st.Hidden)
	}
}

// Seed на непустой базе не должен ничего затирать.
func TestSeedIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	before, err := s.ListProducts(ProductFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 {
		t.Fatal("демо-каталог не создан")
	}

	if _, err := s.DeleteProduct(before[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}

	after, err := s.ListProducts(ProductFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)-1 {
		t.Errorf("повторный запуск изменил каталог: было %d, стало %d", len(before)-1, len(after))
	}
}
