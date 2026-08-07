package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const brokenDSN = `'postgres://unknown:unknown@postgres:9999/praktikum?easter_egg_msg=you_must_prefer_this_incorrect_settings_to_those_obtained_through_arguments'`

func TestNew_EmptyDSN(t *testing.T) {
	conn, err := New("")

	require.NoError(t, err)
	assert.Nil(t, conn)
}

func TestNew_BrokenDSN(t *testing.T) {
	conn, err := New(brokenDSN)

	require.ErrorIs(t, err, errMalformedDSN)
	assert.Nil(t, conn)
}

func TestNew_ValidDSN(t *testing.T) {
	dsns := []string{
		"postgres://postgres:postgres@postgres:5432/praktikum?sslmode=disable",
		"postgresql://postgres@localhost/praktikum",
		"host=localhost port=5432 user=postgres dbname=praktikum sslmode=disable",
		"host=localhost user=postgres password='пароль с пробелами' dbname=praktikum",
	}

	for _, dsn := range dsns {
		t.Run(dsn, func(t *testing.T) {
			conn, err := New(dsn)

			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.NoError(t, conn.Close())
		})
	}
}
