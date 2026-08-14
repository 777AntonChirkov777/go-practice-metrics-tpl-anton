package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
)

type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("unexpected response status %d", e.code)
}

func isRetriable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var statusErr *statusError
	if errors.As(err, &statusErr) {
		return retriableStatus(statusErr.code)
	}

	cause := transportCause(err)

	var dnsErr *net.DNSError
	if errors.As(cause, &dnsErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(cause, &opErr) {
		return true
	}

	var netErr net.Error
	if errors.As(cause, &netErr) && netErr.Timeout() {
		return true
	}

	if errors.Is(cause, syscall.ECONNREFUSED) ||
		errors.Is(cause, syscall.ECONNRESET) ||
		errors.Is(cause, syscall.EPIPE) ||
		errors.Is(cause, syscall.EHOSTUNREACH) {
		return true
	}

	return errors.Is(cause, io.ErrUnexpectedEOF) || errors.Is(cause, io.EOF)
}

func retriableStatus(code int) bool {
	switch {
	case code >= 500 && code <= 599:
		return true
	case code == http.StatusTooManyRequests, code == http.StatusRequestTimeout:
		return true
	default:
		return false
	}
}

func transportCause(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}
	return err
}
