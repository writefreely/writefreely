package db

import (
	"fmt"
	"strings"
)

type ColumnType int

type OptionalInt struct {
	Set   bool
	Value int
}

type OptionalString struct {
	Set   bool
	Value string
}

type SQLBuilder interface {
	ToSQL() (string, error)
}

type Column struct {
	Dialect    DialectType
	Name       string
	Nullable   bool
	Default    OptionalString
	Type       ColumnType
	Size       OptionalInt
	PrimaryKey bool
}

type CreateTableSqlBuilder struct {
	Dialect     DialectType
	Name        string
	IfNotExists bool
	ColumnOrder []string
	Columns     map[string]*Column
	Constraints []string
}

const (
	ColumnTypeBool     ColumnType = iota
	ColumnTypeSmallInt ColumnType = iota
	ColumnTypeInteger  ColumnType = iota
	ColumnTypeChar     ColumnType = iota
	ColumnTypeVarChar  ColumnType = iota
	ColumnTypeText     ColumnType = iota
	ColumnTypeDateTime ColumnType = iota
)

var _ SQLBuilder = &CreateTableSqlBuilder{}

var UnsetSize OptionalInt = OptionalInt{Set: false, Value: 0}
var UnsetDefault OptionalString = OptionalString{Set: false, Value: ""}

func (d ColumnType) Format(dialect DialectType, size OptionalInt) (string, error) {
	if dialect != DialectMySQL && dialect != DialectSQLite {
		return "", fmt.Errorf("unsupported column type %d for dialect %d and size %v", d, dialect, size)
	}
	switch d {
	case ColumnTypeSmallInt:
		{
			if dialect == DialectSQLite {
				return "INTEGER", nil
			}
			mod := ""
			if size.Set {
				mod = fmt.Sprintf("(%d)", size.Value)
			}
			return "SMALLINT" + mod, nil
		}
	case ColumnTypeInteger:
		{
			if dialect == DialectSQLite {
				return "INTEGER", nil
			}
			mod := ""
			if size.Set {
				mod = fmt.Sprintf("(%d)", size.Value)
			}
			return "INT" + mod, nil
		}
	case ColumnTypeChar:
		{
			if dialect == DialectSQLite {
				return "TEXT", nil
			}
			mod := ""
			if size.Set {
				mod = fmt.Sprintf("(%d)", size.Value)
			}
			return "CHAR" + mod, nil
		}
	case ColumnTypeVarChar:
		{
			if dialect == DialectSQLite {
				return "TEXT", nil
			}
			mod := ""
			if size.Set {
				mod = fmt.Sprintf("(%d)", size.Value)
			}
			return "VARCHAR" + mod, nil
		}
	case ColumnTypeBool:
		{
			if dialect == DialectSQLite {
				return "INTEGER", nil
			}
			return "TINYINT(1)", nil
		}
	case ColumnTypeDateTime:
		return "DATETIME", nil
	case ColumnTypeText:
		return "TEXT", nil
	}
	return "", fmt.Errorf("unsupported column type %d for dialect %d and size %v", d, dialect, size)
}

func (c *Column) SetName(name string) *Column {
	c.Name = name
	return c
}

func (c *Column) SetNullable(nullable bool) *Column {
	c.Nullable = nullable
	return c
}

func (c *Column) SetPrimaryKey(pk bool) *Column {
	c.PrimaryKey = pk
	return c
}

func (c *Column) SetDefault(value string) *Column {
	c.Default = OptionalString{Set: true, Value: value}
	return c
}

func (c *Column) SetDefaultCurrentTimestamp() *Column {
	def := "NOW()"
	if c.Dialect == DialectSQLite {
		def = "CURRENT_TIMESTAMP"
	}
	c.Default = OptionalString{Set: true, Value: def}
	return c
}

func (c *Column) SetType(t ColumnType) *Column {
	c.Type = t
	return c
}

func (c *Column) SetSize(size int) *Column {
	c.Size = OptionalInt{Set: true, Value: size}
	return c
}

func (c *Column) String() (string, error) {
	var str strings.Builder

	str.WriteString(c.Name)

	str.WriteString(" ")
	typeStr, err := c.Type.Format(c.Dialect, c.Size)
	if err != nil {
		return "", err
	}

	str.WriteString(typeStr)

	if !c.Nullable {
		str.WriteString(" NOT NULL")
	}

	if c.Default.Set {
		str.WriteString(" DEFAULT ")
		str.WriteString(c.Default.Value)
	}

	if c.PrimaryKey {
		str.WriteString(" PRIMARY KEY")
	}

	return str.String(), nil
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
		columnStr, err := column.String()
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

