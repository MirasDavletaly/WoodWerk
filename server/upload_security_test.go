package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Путь к удаляемому файлу приходит от администратора и может быть каким угодно.
// Проверка обязана удержать его внутри каталога загрузок при любой записи:
// со слэшами, с обратными слэшами и в смешанном виде.
func TestUploadsRelativeRejectsEscapes(t *testing.T) {
	dir := t.TempDir()
	u, err := NewUploads(filepath.Join(dir, "uploads"), 5)
	if err != nil {
		t.Fatal(err)
	}

	bad := []string{
		"/uploads/../secret",
		"/uploads/../../secret",
		"/uploads/a/../../secret",
		// Windows считает обратный слэш разделителем, и filepath.Join
		// раскрывает такие «..» уже после нашей проверки.
		`/uploads/a\..\..\secret`,
		`/uploads/..\secret`,
		`/uploads/a\../..\secret`,
		"/uploads//secret",
		"/etc/passwd",
		"uploads/secret",
		"",
	}
	for _, url := range bad {
		if rel, ok := u.relative(url); ok {
			t.Errorf("адрес %q принят как %q, а должен быть отклонён", url, rel)
		}
	}

	good := map[string]string{
		"/uploads/2026/08/a.jpg": "2026/08/a.jpg",
		"/uploads/a.png":         "a.png",
	}
	for url, want := range good {
		rel, ok := u.relative(url)
		if !ok || rel != want {
			t.Errorf("адрес %q дал %q, %v; ожидалось %q", url, rel, ok, want)
		}
	}
}

// Delete не должен трогать ничего за пределами каталога загрузок.
func TestUploadsDeleteStaysInsideDirectory(t *testing.T) {
	root := t.TempDir()
	uploadsDir := filepath.Join(root, "uploads")
	u, err := NewUploads(uploadsDir, 5)
	if err != nil {
		t.Fatal(err)
	}

	// Файл рядом с каталогом загрузок — до него дотягиваться нельзя.
	outside := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(outside, []byte("важное"), 0o600); err != nil {
		t.Fatal(err)
	}

	attempts := []string{
		`/uploads/..\secret.txt`,
		`/uploads/a\..\..\secret.txt`,
		"/uploads/../secret.txt",
	}
	for _, url := range attempts {
		if err := u.Delete(url); err != nil {
			t.Fatalf("Delete(%q) вернул ошибку: %v", url, err)
		}
		if _, err := os.Stat(outside); os.IsNotExist(err) {
			t.Fatalf("Delete(%q) удалил файл за пределами каталога загрузок", url)
		}
	}

	// А свой файл удалиться обязан.
	own := filepath.Join(uploadsDir, "2026", "08")
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(own, "pic.jpg")
	if err := os.WriteFile(target, []byte("картинка"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := u.Delete("/uploads/2026/08/pic.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("свой файл не удалился")
	}
}
