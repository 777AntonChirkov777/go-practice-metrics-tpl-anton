package dbstore

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"testing"

	"practice/internal/storage"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsRetriable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{
			name: "class 08 connection_exception",
			err:  &pgconn.PgError{Code: pgerrcode.ConnectionException},
			want: true,
		},
		{
			name: "class 08 connection_does_not_exist",
			err:  &pgconn.PgError{Code: pgerrcode.ConnectionDoesNotExist},
			want: true,
		},
		{
			name: "class 08 connection_failure",
			err:  &pgconn.PgError{Code: pgerrcode.ConnectionFailure},
			want: true,
		},
		{
			name: "unique_violation",
			err:  &pgconn.PgError{Code: pgerrcode.UniqueViolation},
			want: false,
		},
		{
			name: "syntax_error",
			err:  &pgconn.PgError{Code: pgerrcode.SyntaxError},
			want: false,
		},
		{
			name: "обёрнутая ошибка класса 08",
			err:  fmt.Errorf("save batch: %w", &pgconn.PgError{Code: pgerrcode.ConnectionFailure}),
			want: true,
		},
		{
			name: "обёрнутое нарушение уникальности",
			err:  fmt.Errorf("save batch: %w", &pgconn.PgError{Code: pgerrcode.UniqueViolation}),
			want: false,
		},
		{
			name: "соединение не установлено",
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			want: true,
		},
		{
			name: "обёрнутая сетевая ошибка",
			err:  fmt.Errorf("connect: %w", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}),
			want: true,
		},
		{name: "driver.ErrBadConn", err: driver.ErrBadConn, want: true},
		{name: "sql.ErrConnDone", err: sql.ErrConnDone, want: true},
		{name: "sql.ErrNoRows", err: sql.ErrNoRows, want: false},
		{name: "storage.ErrNotFound", err: storage.ErrNotFound, want: false},
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

func TestIsRetriable_SQLStateWinsOverWrappedNetworkError(t *testing.T) {
	err := fmt.Errorf("%w: %w",
		&pgconn.PgError{Code: pgerrcode.UniqueViolation},
		&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")})

	if isRetriable(err) {
		t.Error("код SQLSTATE вне класса 08 обязан побеждать сетевую ошибку рядом с ним")
	}
}
