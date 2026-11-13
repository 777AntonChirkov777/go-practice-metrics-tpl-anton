package handlers

import (
	"net"
	"net/http"
)

// Auditor описывает приёмник событий аудита, который хендлеры уведомляют
// после успешного сохранения метрик. В сервисе его реализует audit.Notifier.
type Auditor interface {
	// Enabled сообщает, есть ли у аудита хотя бы один получатель событий.
	// Хендлеры вызывают [Auditor.Record] только при true.
	Enabled() bool
	// Record регистрирует событие аудита: metrics — имена сохранённых метрик,
	// ip — хост из RemoteAddr запроса без порта; если RemoteAddr не разбирается
	// как host:port, он передаётся целиком.
	Record(metrics []string, ip string)
}

func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}

	return host
}
