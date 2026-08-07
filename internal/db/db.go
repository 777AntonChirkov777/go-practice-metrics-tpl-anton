package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	maxOpenConns    = 10
	maxIdleConns    = 5
	connMaxLifetime = 5 * time.Minute
)

var errMalformedDSN = errors.New("некорректная строка подключения к базе данных: ожидается URL postgres:// или пары key=value")

func New(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil
	}

	if err := validateDSN(dsn); err != nil {
		return nil, err
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetConnMaxLifetime(connMaxLifetime)

	return conn, nil
}

func validateDSN(dsn string) error {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return nil
	}

	tokens := strings.Fields(dsn)
	if len(tokens) == 0 {
		return errMalformedDSN
	}

	key, _, found := strings.Cut(tokens[0], "=")
	if !found || !isKeywordKey(key) {
		return errMalformedDSN
	}

	return nil
}

func isKeywordKey(key string) bool {
	if key == "" {
		return false
	}

	for _, r := range key {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '_' {
			return false
		}
	}

	return true
}
