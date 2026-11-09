package handlers

import (
	"net"
	"net/http"
)

type Auditor interface {
	Enabled() bool
	Record(metrics []string, ip string)
}

func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}

	return host
}
