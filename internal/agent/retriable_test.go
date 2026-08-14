package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"syscall"
	"testing"
)

func TestIsRetriable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{
			name: "соединение не установлено",
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			want: true,
		},
		{
			name: "имя сервера не разрешилось",
			err:  &net.DNSError{Err: "no such host", Name: "metrics.invalid"},
			want: true,
		},
		{
			name: "соединение сброшено",
			err:  syscall.ECONNRESET,
			want: true,
		},
		{
			name: "ответ оборван на середине",
			err:  io.ErrUnexpectedEOF,
			want: true,
		},
		{
			name: "обёртка url.Error вокруг сетевой ошибки",
			err: &url.Error{
				Op:  "Post",
				URL: "http://metrics.invalid/updates/",
				Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			},
			want: true,
		},
		{
			name: "отмена контекста в обёртке url.Error",
			err: &url.Error{
				Op:  "Post",
				URL: "http://metrics.invalid/updates/",
				Err: context.Canceled,
			},
			want: false,
		},
		{
			name: "истёк дедлайн контекста",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{name: "статус 500", err: &statusError{code: 500}, want: true},
		{name: "статус 503", err: &statusError{code: 503}, want: true},
		{name: "статус 429", err: &statusError{code: 429}, want: true},
		{name: "статус 408", err: &statusError{code: 408}, want: true},
		{name: "статус 400", err: &statusError{code: 400}, want: false},
		{name: "статус 404", err: &statusError{code: 404}, want: false},
		{
			name: "обёрнутый статус 500",
			err:  fmt.Errorf("send batch: %w", &statusError{code: 500}),
			want: true,
		},
		{name: "произвольная ошибка", err: errors.New("boom"), want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isRetriable(c.err); got != c.want {
				t.Errorf("isRetriable(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestIsRetriable_RequestBuildErrorIsPermanent(t *testing.T) {
	_, err := url.Parse("http://\x7f/updates/")
	if err == nil {
		t.Fatal("ожидалась ошибка разбора адреса с управляющим символом")
	}

	if isRetriable(err) {
		t.Error("ошибка построения запроса вечная, повторять её нельзя")
	}
}
