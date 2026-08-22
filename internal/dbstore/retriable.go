package dbstore

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func isRetriable(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgerrcode.IsConnectionException(pgErr.Code)
	}

	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone)
}
