package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var fsys embed.FS

// Up применяет к базе db все ещё не применённые миграции из встроенных файлов
// *.sql, используя goose с диалектом postgres. Up меняет глобальные настройки
// goose: файловую систему, журнал (он отключается) и диалект. При повторном
// вызове уже применённые миграции пропускаются. Возвращает первую возникшую
// ошибку.
func Up(db *sql.DB) error {
	goose.SetBaseFS(fsys)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.Up(db, ".")
}
