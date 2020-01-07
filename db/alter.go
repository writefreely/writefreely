package db

import (
	"fmt"
	"strings"
)

type AlterTableSqlBuilder struct {
	Dialect DialectType
	Name    string
	Changes []string
}

func (b *AlterTableSqlBuilder) AddColumn(col *Column) *AlterTableSqlBuilder {
	if colVal, err := col.String(); err == nil {
		b.Changes = append(b.Changes, fmt.Sprintf("ADD COLUMN %s", colVal))
	}
	return b
}

func (b *AlterTableSqlBuilder) ChangeColumn(name string, col *Column) *AlterTableSqlBuilder {
	if colVal, err := col.String(); err == nil {
		b.Changes = append(b.Changes, fmt.Sprintf("CHANGE COLUMN %s %s", name, colVal))
	}
	return b
}

func (b *AlterTableSqlBuilder) AddUniqueConstraint(name string, columns ...string) *AlterTableSqlBuilder {
	b.Changes = append(b.Changes, fmt.Sprintf("ADD CONSTRAINT %s UNIQUE (%s)", name, strings.Join(columns, ", ")))
	return b
}

func (b *AlterTableSqlBuilder) ToSQL() (string, error) {
	var str strings.Builder

	str.WriteString("ALTER TABLE ")
	str.WriteString(b.Name)
	str.WriteString(" ")

	if len(b.Changes) == 0 {
		return "", fmt.Errorf("no changes provide for table: %s", b.Name)
	}
	changeCount := len(b.Changes)
	for i, thing := range b.Changes {
		str.WriteString(thing)
		if i < changeCount-1 {
			str.WriteString(", ")
		}
	}

	return str.String(), nil
}
