package db

import "fmt"

type DialectType int

const (
	DialectSQLite DialectType = iota
	DialectMySQL  DialectType = iota
)

func (d DialectType) IsKnown() bool {
	switch d {
	case DialectSQLite, DialectMySQL:
		return true
	default:
		return false
	}
}

func (d DialectType) AssertKnown() {
	if !d.IsKnown() {
		panic(fmt.Sprintf("unexpected dialect: %d", d))
	}
}

func (d DialectType) Table(name string) *CreateTableSqlBuilder {
	d.AssertKnown()
	return &CreateTableSqlBuilder{Dialect: d, Name: name}
}

func (d DialectType) AlterTable(name string) *AlterTableSqlBuilder {
	d.AssertKnown()
	return &AlterTableSqlBuilder{Dialect: d, Name: name}
}

func (d DialectType) CreateUniqueIndex(name, table string, columns ...string) *CreateIndexSqlBuilder {
	d.AssertKnown()
	return &CreateIndexSqlBuilder{Dialect: d, Name: name, Table: table, Unique: true, Columns: columns}
}

func (d DialectType) CreateIndex(name, table string, columns ...string) *CreateIndexSqlBuilder {
	d.AssertKnown()
	return &CreateIndexSqlBuilder{Dialect: d, Name: name, Table: table, Unique: false, Columns: columns}
}

func (d DialectType) DropIndex(name, table string) *DropIndexSqlBuilder {
	d.AssertKnown()
	return &DropIndexSqlBuilder{Dialect: d, Name: name, Table: table}
}
