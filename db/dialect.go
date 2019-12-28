package db

import "fmt"

type DialectType int

const (
	DialectSQLite DialectType = iota
	DialectMySQL  DialectType = iota
)

func (d DialectType) Column(name string, t ColumnType, size OptionalInt) *Column {
	switch d {
	case DialectSQLite:
		return &Column{Dialect: DialectSQLite, Name: name, Type: t, Size: size}
	case DialectMySQL:
		return &Column{Dialect: DialectMySQL, Name: name, Type: t, Size: size}
	default:
		panic(fmt.Sprintf("unexpected dialect: %d", d))
	}
}

func (d DialectType) Table(name string) *CreateTableSqlBuilder {
	switch d {
	case DialectSQLite:
		return &CreateTableSqlBuilder{Dialect: DialectSQLite, Name: name}
	case DialectMySQL:
		return &CreateTableSqlBuilder{Dialect: DialectMySQL, Name: name}
	default:
		panic(fmt.Sprintf("unexpected dialect: %d", d))
	}
}

func (d DialectType) AlterTable(name string) *AlterTableSqlBuilder {
	switch d {
	case DialectSQLite:
		return &AlterTableSqlBuilder{Dialect: DialectSQLite, Name: name}
	case DialectMySQL:
		return &AlterTableSqlBuilder{Dialect: DialectMySQL, Name: name}
	default:
		panic(fmt.Sprintf("unexpected dialect: %d", d))
	}
}
