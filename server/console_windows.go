//go:build windows

package main

import "syscall"

// Консоль Windows с русской локалью по умолчанию работает в кодировке 866,
// а Go пишет в UTF-8 — русский текст превращается в кракозябры. Читать оттуда
// пароль администратора становится невозможно, поэтому переключаем вывод
// консоли на UTF-8 сразу при запуске.
//
// Если вывод перенаправлен в файл или консоли нет вообще, вызов просто
// вернёт ошибку, и мы её игнорируем: ничего не сломается.
func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	const utf8CodePage = 65001
	_, _, _ = setConsoleOutputCP.Call(uintptr(utf8CodePage))
}
