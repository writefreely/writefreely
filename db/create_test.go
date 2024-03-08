package db

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateTableSqlBuilder_ToSQL(t *testing.T) {
	sql, err := DialectMySQL.
		Table("foo").
		SetIfNotExists(true).
		Column(PrimaryKeyColumn("bar", ColumnTypeInt{MaxBytes: 4})).
		Column(NonNullableColumn("baz", ColumnTypeString{})).
		Column(NonNullableColumn("qux", ColumnTypeDateTime{DefaultVal: DefaultNow})).
		UniqueConstraint("bar").
		UniqueConstraint("bar", "baz").
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "CREATE TABLE IF NOT EXISTS foo ( bar INT NOT NULL PRIMARY KEY, baz TEXT NOT NULL, qux DATETIME NOT NULL DEFAULT NOW(), UNIQUE(bar), UNIQUE(bar,baz) )", sql)
}
