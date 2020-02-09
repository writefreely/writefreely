package db

import (
	"fmt"
	"strings"
)

type CreateIndexSqlBuilder struct {
	Dialect DialectType
	Name    string
	Table   string
	Unique  bool
	Columns []string
}

type DropIndexSqlBuilder struct {
	Dialect DialectType
	Name    string
	Table   string
}

func (b *CreateIndexSqlBuilder) ToSQL() (string, error) {
	var str strings.Builder

	str.WriteString("CREATE ")
	if b.Unique {
		str.WriteString("UNIQUE ")
	}
	str.WriteString("INDEX ")
	str.WriteString(b.Name)
	str.WriteString(" on ")
	str.WriteString(b.Table)

	if len(b.Columns) == 0 {
		return "", fmt.Errorf("columns provided for this index: %s", b.Name)
	}

	str.WriteString(" (")
	columnCount := len(b.Columns)
	for i, thing := range b.Columns {
		str.WriteString(thing)
		if i < columnCount-1 {
			str.WriteString(", ")
		}
	}
	str.WriteString(")")

	return str.String(), nil
}

func (b *DropIndexSqlBuilder) ToSQL() (string, error) {
	return fmt.Sprintf("DROP INDEX %s on %s", b.Name, b.Table), nil
}
