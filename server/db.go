// Хранилище WOODWERK на SQLite.
//
// Драйвер modernc.org/sqlite — чистый Go, без cgo: собирается и работает
// на любом сервере без установки библиотек. Вся база — один файл,
// резервная копия делается обычным копированием.
package main

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// schema создаётся при каждом запуске: IF NOT EXISTS делает вызов безопасным.
const schema = `
CREATE TABLE IF NOT EXISTS categories (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    name_kk    TEXT    NOT NULL DEFAULT '',
    name_en    TEXT    NOT NULL DEFAULT '',
    slug       TEXT    NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    price       INTEGER NOT NULL DEFAULT 0,
    image_url   TEXT    NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    status      TEXT    NOT NULL DEFAULT 'active',
    size        TEXT    NOT NULL DEFAULT '',
    badge       TEXT    NOT NULL DEFAULT '',
    -- Переводы необязательны: пустое поле означает «показывать русский».
    title_kk       TEXT NOT NULL DEFAULT '',
    title_en       TEXT NOT NULL DEFAULT '',
    description_kk TEXT NOT NULL DEFAULT '',
    description_en TEXT NOT NULL DEFAULT '',
    -- Название и описание в нижнем регистре: LOWER() в SQLite знает только
    -- латиницу, поэтому регистр русских букв приводим в Go и храним готовым.
    search_text TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_products_category ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_status   ON products(status);

CREATE TABLE IF NOT EXISTS product_images (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    image_url  TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_product_images_product ON product_images(product_id);

-- Галерея «Панели в интерьере» на главной. Раньше карточки были свёрстаны
-- прямо в index.html; теперь их ведёт администратор.
CREATE TABLE IF NOT EXISTS gallery (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    image_url  TEXT    NOT NULL,
    alt        TEXT    NOT NULL DEFAULT '',
    title      TEXT    NOT NULL,
    title_kk   TEXT    NOT NULL DEFAULT '',
    title_en   TEXT    NOT NULL DEFAULT '',
    caption    TEXT    NOT NULL DEFAULT '',
    caption_kk TEXT    NOT NULL DEFAULT '',
    caption_en TEXT    NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    visible    INTEGER NOT NULL DEFAULT 1,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gallery_order ON gallery(sort_order);

CREATE TABLE IF NOT EXISTS admin_users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    csrf       TEXT    NOT NULL,
    created_at TEXT    NOT NULL,
    expires_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
`

// openDB открывает файл базы, включает нужные режимы и накатывает схему.
func openDB(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	// WAL — чтение не блокирует запись; busy_timeout — вместо ошибки "database
	// is locked" драйвер подождёт; foreign_keys — чтобы каскады реально работали.
	dsn := filepath.ToSlash(path) +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Записей мало, а одно соединение полностью снимает вопрос блокировок.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// migrate дополняет таблицы колонками, появившимися позже схемы.
// Нужна для баз, созданных прошлыми версиями сервера.
func migrate(db *sql.DB) error {
	// Каталог переехал с мебели на отделочные панели: колонка «порода дерева»
	// стала «размером панели». На старых базах переименовываем её на месте,
	// чтобы значения и индексы остались прежними.
	hasWood, err := hasColumn(db, "products", "wood")
	if err != nil {
		return err
	}
	hasSize, err := hasColumn(db, "products", "size")
	if err != nil {
		return err
	}
	if hasWood && !hasSize {
		if _, err := db.Exec(`ALTER TABLE products RENAME COLUMN wood TO size`); err != nil {
			return err
		}
	}

	// Колонки переводов появились позже — дозаводим их на старых базах.
	translated := []struct {
		table  string
		column string
	}{
		{"products", "title_kk"},
		{"products", "title_en"},
		{"products", "description_kk"},
		{"products", "description_en"},
		{"categories", "name_kk"},
		{"categories", "name_en"},
	}
	for _, c := range translated {
		has, err := hasColumn(db, c.table, c.column)
		if err != nil {
			return err
		}
		if !has {
			stmt := "ALTER TABLE " + c.table + " ADD COLUMN " + c.column +
				" TEXT NOT NULL DEFAULT ''"
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}
	}

	has, err := hasColumn(db, "products", "search_text")
	if err != nil {
		return err
	}
	if !has {
		if _, err := db.Exec(`ALTER TABLE products ADD COLUMN search_text TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
		// Заполним поле для уже существующих изделий.
		rows, err := db.Query(`SELECT id, title, description FROM products`)
		if err != nil {
			return err
		}
		type item struct {
			id    int64
			title string
			desc  string
		}
		var items []item
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.id, &it.title, &it.desc); err != nil {
				rows.Close()
				return err
			}
			items = append(items, it)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, it := range items {
			if _, err := db.Exec(`UPDATE products SET search_text = ? WHERE id = ?`,
				searchText(it.title, it.desc), it.id); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
