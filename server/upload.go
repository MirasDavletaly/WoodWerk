// Загрузка фотографий мебели.
//
// Файлы кладём на диск в uploads/ГГГГ/ММ/ со случайным именем, а в базе
// храним только адрес вида /uploads/2026/08/ab12….jpg. Тип определяем по
// содержимому файла, а не по расширению и не по заголовку от браузера.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// uploadPrefix — публичный адрес, по которому отдаются загруженные файлы.
const uploadPrefix = "/uploads/"

// allowedImages — какие форматы принимаем и с каким расширением сохраняем.
var allowedImages = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// ErrBadImage — файл не похож на поддерживаемое изображение.
var ErrBadImage = errors.New("подойдут только изображения JPG, PNG, WEBP или GIF")

// ErrTooBig — файл больше разрешённого размера.
var ErrTooBig = errors.New("файл слишком большой")

// Uploads отвечает за папку с фотографиями.
type Uploads struct {
	dir      string
	maxBytes int64
}

func NewUploads(dir string, maxMB int) (*Uploads, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	if maxMB < 1 {
		maxMB = 5
	}
	return &Uploads{dir: abs, maxBytes: int64(maxMB) << 20}, nil
}

// MaxBytes — предел размера одного файла, нужен фронтенду для подсказки.
func (u *Uploads) MaxBytes() int64 { return u.maxBytes }

// Save проверяет и сохраняет одну картинку, возвращая её публичный адрес.
func (u *Uploads) Save(fh *multipart.FileHeader) (string, error) {
	if fh.Size > u.maxBytes {
		return "", ErrTooBig
	}

	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Читаем на байт больше предела: так поймаем файл с заниженным Size.
	data, err := io.ReadAll(io.LimitReader(src, u.maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > u.maxBytes {
		return "", ErrTooBig
	}
	if len(data) == 0 {
		return "", ErrBadImage
	}

	// Тип берём из содержимого: расширению и Content-Type от клиента не верим.
	kind := http.DetectContentType(data)
	if i := strings.IndexByte(kind, ';'); i > 0 {
		kind = strings.TrimSpace(kind[:i])
	}
	ext, ok := allowedImages[kind]
	if !ok {
		return "", ErrBadImage
	}

	name, err := randomName(ext)
	if err != nil {
		return "", err
	}

	now := time.Now()
	rel := path.Join(now.Format("2006"), now.Format("01"), name)
	full := filepath.Join(u.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", err
	}
	return uploadPrefix + rel, nil
}

func randomName(ext string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + ext, nil
}

// Delete убирает файл с диска по его публичному адресу.
// Всё, что лежит вне uploads/, игнорируем: удалять картинки сайта нельзя.
func (u *Uploads) Delete(url string) error {
	rel, ok := u.relative(url)
	if !ok {
		return nil
	}
	err := os.Remove(filepath.Join(u.dir, filepath.FromSlash(rel)))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// relative проверяет адрес и превращает его в путь внутри папки загрузок.
//
// Проверка синтаксиса тут не единственная: в конце мы раскрываем путь
// целиком и убеждаемся, что он действительно лежит внутри каталога.
// Одной проверке символов доверять нельзя — Windows считает разделителем
// и обратный слэш, поэтому «a\..\..\secret» выглядит как имя одного файла,
// а filepath.Join раскрывает его наружу.
func (u *Uploads) relative(url string) (string, bool) {
	if !strings.HasPrefix(url, uploadPrefix) {
		return "", false
	}

	rest := strings.TrimPrefix(url, uploadPrefix)

	// В адресе картинки обратному слэшу и управляющим символам делать нечего.
	if strings.ContainsAny(rest, "\\\x00") {
		return "", false
	}

	rel := path.Clean(rest)
	if rel == "." || rel == "/" || strings.HasPrefix(rel, "..") || path.IsAbs(rel) {
		return "", false
	}
	for _, part := range strings.Split(rel, "/") {
		if part == ".." || part == "" {
			return "", false
		}
	}

	// Последнее слово за файловой системой: раскрываем путь и сверяем,
	// что он не вышел за пределы каталога загрузок.
	full := filepath.Join(u.dir, filepath.FromSlash(rel))
	inside, err := filepath.Rel(u.dir, full)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", false
	}

	return rel, true
}

// Handler отдаёт загруженные файлы. Листинг папок закрыт.
func (u *Uploads) Handler() http.Handler {
	files := http.FileServer(http.Dir(u.dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		if _, ok := u.relative(path.Clean(r.URL.Path)); !ok {
			http.NotFound(w, r)
			return
		}
		// Имена файлов случайные и не меняются — можно кэшировать надолго.
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.StripPrefix(uploadPrefix, files).ServeHTTP(w, r)
	})
}

// humanSize нужен только для текста ошибки.
func humanSize(b int64) string {
	return fmt.Sprintf("%.0f МБ", float64(b)/(1<<20))
}
