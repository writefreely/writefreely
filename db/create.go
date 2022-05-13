/*
 * Copyright © 2019-2020 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package db

import (
	"fmt"
	"strings"
)

type CreateTableSqlBuilder struct {
	Dialect     DialectType
	Name        string
	IfNotExists bool
	ColumnOrder []string
	Columns     map[string]*Column
	Constraints []string
}

func (b *CreateTableSqlBuilder) Column(column *Column) *CreateTableSqlBuilder {
	if b.Columns == nil {
		b.Columns = make(map[string]*Column)
	}
	b.Columns[column.Name] = column
	b.ColumnOrder = append(b.ColumnOrder, column.Name)
	return b
}

func (b *CreateTableSqlBuilder) UniqueConstraint(columns ...string) *CreateTableSqlBuilder {
	for _, column := range columns {
		if _, ok := b.Columns[column]; !ok {
			// This fails silently.
			return b
		}
	}
	b.Constraints = append(b.Constraints, fmt.Sprintf("UNIQUE(%s)", strings.Join(columns, ",")))
	return b
}

func (b *CreateTableSqlBuilder) SetIfNotExists(ine bool) *CreateTableSqlBuilder {
	b.IfNotExists = ine
	return b
}

func (b *CreateTableSqlBuilder) ToSQL() (string, error) {
	var str strings.Builder

	str.WriteString("CREATE TABLE ")
	if b.IfNotExists {
		str.WriteString("IF NOT EXISTS ")
	}
	str.WriteString(b.Name)

	var things []string
	for _, columnName := range b.ColumnOrder {
		column, ok := b.Columns[columnName]
		if !ok {
			return "", fmt.Errorf("column not found: %s", columnName)
		}
		columnStr, err := column.CreateSQL(b.Dialect)
		if err != nil {
			return "", err
		}
		things = append(things, columnStr)
	}
	for _, constraint := range b.Constraints {
		things = append(things, constraint)
	}

	if thingLen := len(things); thingLen > 0 {
		str.WriteString(" ( ")
		for i, thing := range things {
			str.WriteString(thing)
			if i < thingLen-1 {
				str.WriteString(", ")
			}
		}
		str.WriteString(" )")
	}

	return str.String(), nil
}
