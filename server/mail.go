// Отправка заявок на почту.
//
// Письмо — не единственный путь заявки: она в любом случае дописывается
// в leads.jsonl. Почта добавляется сверху, и её отказ никогда не должен
// оборачиваться ошибкой для посетителя — иначе сорванная отправка
// выглядела бы как неработающая форма.
package main

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// mailer хранит доступ к почтовому серверу. Пустой Host означает, что
// отправка выключена: сайт работает как раньше, только с файлом.
type mailer struct {
	Host string
	Port string
	User string
	Pass string
	To   string
}

// envOr читает переменную окружения, подставляя значение по умолчанию.
func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func (m *mailer) enabled() bool {
	return m != nil && m.Host != "" && m.User != "" && m.Pass != "" && m.To != ""
}

// Send отправляет письмо о новой заявке. Вызывать в отдельной горутине:
// SMTP отвечает не мгновенно, а посетителю ждать незачем.
func (m *mailer) Send(item storedLead) {
	if !m.enabled() {
		return
	}

	subject := fmt.Sprintf("Заявка с сайта: %s, %s", item.Name, item.Phone)

	var body strings.Builder
	fmt.Fprintf(&body, "Имя: %s\r\n", item.Name)
	fmt.Fprintf(&body, "Телефон: %s\r\n", item.Phone)
	if item.Type != "" {
		fmt.Fprintf(&body, "Интересует: %s\r\n", item.Type)
	}
	if item.Comment != "" {
		fmt.Fprintf(&body, "Комментарий: %s\r\n", item.Comment)
	}
	fmt.Fprintf(&body, "\r\nВремя: %s\r\nIP: %s\r\n", item.At, item.IP)

	// Тема письма с кириллицей без кодирования превращается в мусор
	// в большинстве почтовых клиентов.
	msg := "From: " + m.User + "\r\n" +
		"To: " + m.To + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n" + body.String()

	if err := m.send([]byte(msg)); err != nil {
		// Заявка уже лежит в файле, поэтому здесь только запись в журнал.
		logError(fmt.Errorf("не удалось отправить письмо о заявке: %w", err))
	}
}

func (m *mailer) send(msg []byte) error {
	addr := net.JoinHostPort(m.Host, m.Port)

	// Свой таймаут: без него зависший почтовый сервер держал бы горутину
	// неопределённо долго.
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return err
	}
	defer c.Close()

	// Порт 587 начинает открытым соединением и поднимает шифрование
	// командой STARTTLS; на 465 канал зашифрован с самого начала.
	if m.Port != "465" {
		if err := c.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
			return err
		}
	}

	if err := c.Auth(smtp.PlainAuth("", m.User, m.Pass, m.Host)); err != nil {
		return err
	}
	if err := c.Mail(m.User); err != nil {
		return err
	}
	for _, to := range strings.Split(m.To, ",") {
		to = strings.TrimSpace(to)
		if to == "" {
			continue
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
