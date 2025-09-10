package db

import (
	"fmt"
	"strings"
)

type alterTableQueryType int

const (
	alter  alterTableQueryType = iota
	update alterTableQueryType = iota
)

type change struct {
	queryType   alterTableQueryType
	queryString string
}

type AlterTableSqlBuilder struct {
	Dialect DialectType
	Name    string
	Changes []change
}

func (b *AlterTableSqlBuilder) AddColumn(col *Column) *AlterTableSqlBuilder {
	if colVal, err := col.String(); err == nil {
		b.Changes = append(b.Changes, change{queryType: alter, queryString: fmt.Sprintf("ADD COLUMN %s", colVal)})
	}
	return b
}

func (b *AlterTableSqlBuilder) ChangeColumn(name string, col *Column) *AlterTableSqlBuilder {
	if colVal, err := col.String(); err == nil {
		b.Changes = append(b.Changes, change{queryType: alter, queryString: fmt.Sprintf("RENAME COLUMN %s TO %s_old", name, name)})
		b.Changes = append(b.Changes, change{queryType: alter, queryString: fmt.Sprintf("ADD COLUMN %s", colVal)})
		b.Changes = append(b.Changes, change{queryType: update, queryString: fmt.Sprintf("SET %s = %s_old", col.Name, name)})
		b.Changes = append(b.Changes, change{queryType: alter, queryString: fmt.Sprintf("DROP COLUMN %s_old", name)})
	}
	return b
}

func (b *AlterTableSqlBuilder) AddUniqueConstraint(name string, columns ...string) *AlterTableSqlBuilder {
	b.Changes = append(b.Changes, change{
		queryType: alter,
		queryString: fmt.Sprintf("ADD CONSTRAINT %s UNIQUE (%s)", name, strings.Join(columns, ", ")),
	})
	return b
}

func (b *AlterTableSqlBuilder) ToSQL() (string, error) {
	str := ""
	changeCount := len(b.Changes)

	if changeCount == 0 {
		return "", fmt.Errorf("no changes provide for table: %s", b.Name)
	}

	for i, change := range b.Changes {
		switch change.queryType {
		case alter:
			str += fmt.Sprintf("ALTER TABLE %s ", b.Name)
		case update:
			str += fmt.Sprintf("UPDATE %s ", b.Name)
		}
		str += change.queryString
		if i < changeCount - 1 {
			str += "; "
		}
	}

	return str, nil
}
