package db

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDialect_Column(t *testing.T) {
	c1 := DialectSQLite.Column("foo", ColumnTypeBool, UnsetSize)
	assert.Equal(t, DialectSQLite, c1.Dialect)
	c2 := DialectMySQL.Column("foo", ColumnTypeBool, UnsetSize)
	assert.Equal(t, DialectMySQL, c2.Dialect)
}

func TestColumnType_Format(t *testing.T) {
	type args struct {
		dialect DialectType
		size    OptionalInt
	}
	tests := []struct {
		name    string
		d       ColumnType
		args    args
		want    string
		wantErr bool
	}{
		{"Sqlite bool", ColumnTypeBool, args{dialect: DialectSQLite}, "INTEGER", false},
		{"Sqlite small int", ColumnTypeSmallInt, args{dialect: DialectSQLite}, "INTEGER", false},
		{"Sqlite int", ColumnTypeInteger, args{dialect: DialectSQLite}, "INTEGER", false},
		{"Sqlite char", ColumnTypeChar, args{dialect: DialectSQLite}, "TEXT", false},
		{"Sqlite varchar", ColumnTypeVarChar, args{dialect: DialectSQLite}, "TEXT", false},
		{"Sqlite text", ColumnTypeText, args{dialect: DialectSQLite}, "TEXT", false},
		{"Sqlite datetime", ColumnTypeDateTime, args{dialect: DialectSQLite}, "DATETIME", false},

		{"MySQL bool", ColumnTypeBool, args{dialect: DialectMySQL}, "TINYINT(1)", false},
		{"MySQL small int", ColumnTypeSmallInt, args{dialect: DialectMySQL}, "SMALLINT", false},
		{"MySQL small int with param", ColumnTypeSmallInt, args{dialect: DialectMySQL, size: OptionalInt{true, 3}}, "SMALLINT(3)", false},
		{"MySQL int", ColumnTypeInteger, args{dialect: DialectMySQL}, "INT", false},
		{"MySQL int with param", ColumnTypeInteger, args{dialect: DialectMySQL, size: OptionalInt{true, 11}}, "INT(11)", false},
		{"MySQL char", ColumnTypeChar, args{dialect: DialectMySQL}, "CHAR", false},
		{"MySQL char with param", ColumnTypeChar, args{dialect: DialectMySQL, size: OptionalInt{true, 4}}, "CHAR(4)", false},
		{"MySQL varchar", ColumnTypeVarChar, args{dialect: DialectMySQL}, "VARCHAR", false},
		{"MySQL varchar with param", ColumnTypeVarChar, args{dialect: DialectMySQL, size: OptionalInt{true, 25}}, "VARCHAR(25)", false},
		{"MySQL text", ColumnTypeText, args{dialect: DialectMySQL}, "TEXT", false},
		{"MySQL datetime", ColumnTypeDateTime, args{dialect: DialectMySQL}, "DATETIME", false},

		{"invalid column type", 10000, args{dialect: DialectMySQL}, "", true},
		{"invalid dialect", ColumnTypeBool, args{dialect: 10000}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.d.Format(tt.args.dialect, tt.args.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Format() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColumn_Build(t *testing.T) {
	type fields struct {
		Dialect    DialectType
		Name       string
		Nullable   bool
		Default    OptionalString
		Type       ColumnType
		Size       OptionalInt
		PrimaryKey bool
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{"Sqlite bool", fields{DialectSQLite, "foo", false, UnsetDefault, ColumnTypeBool, UnsetSize, false}, "foo INTEGER NOT NULL", false},
		{"Sqlite bool nullable", fields{DialectSQLite, "foo", true, UnsetDefault, ColumnTypeBool, UnsetSize, false}, "foo INTEGER", false},
		{"Sqlite small int", fields{DialectSQLite, "foo", false, UnsetDefault, ColumnTypeSmallInt, UnsetSize, true}, "foo INTEGER NOT NULL PRIMARY KEY", false},
		{"Sqlite small int nullable", fields{DialectSQLite, "foo", true, UnsetDefault, ColumnTypeSmallInt, UnsetSize, false}, "foo INTEGER", false},
		{"Sqlite int", fields{DialectSQLite, "foo", false, UnsetDefault, ColumnTypeInteger, UnsetSize, false}, "foo INTEGER NOT NULL", false},
		{"Sqlite int nullable", fields{DialectSQLite, "foo", true, UnsetDefault, ColumnTypeInteger, UnsetSize, false}, "foo INTEGER", false},
		{"Sqlite char", fields{DialectSQLite, "foo", false, UnsetDefault, ColumnTypeChar, UnsetSize, false}, "foo TEXT NOT NULL", false},
		{"Sqlite char nullable", fields{DialectSQLite, "foo", true, UnsetDefault, ColumnTypeChar, UnsetSize, false}, "foo TEXT", false},
		{"Sqlite varchar", fields{DialectSQLite, "foo", false, UnsetDefault, ColumnTypeVarChar, UnsetSize, false}, "foo TEXT NOT NULL", false},
		{"Sqlite varchar nullable", fields{DialectSQLite, "foo", true, UnsetDefault, ColumnTypeVarChar, UnsetSize, false}, "foo TEXT", false},
		{"Sqlite text", fields{DialectSQLite, "foo", false, UnsetDefault, ColumnTypeText, UnsetSize, false}, "foo TEXT NOT NULL", false},
		{"Sqlite text nullable", fields{DialectSQLite, "foo", true, UnsetDefault, ColumnTypeText, UnsetSize, false}, "foo TEXT", false},
		{"Sqlite datetime", fields{DialectSQLite, "foo", false, UnsetDefault, ColumnTypeDateTime, UnsetSize, false}, "foo DATETIME NOT NULL", false},
		{"Sqlite datetime nullable", fields{DialectSQLite, "foo", true, UnsetDefault, ColumnTypeDateTime, UnsetSize, false}, "foo DATETIME", false},

		{"MySQL bool", fields{DialectMySQL, "foo", false, UnsetDefault, ColumnTypeBool, UnsetSize, false}, "foo TINYINT(1) NOT NULL", false},
		{"MySQL bool nullable", fields{DialectMySQL, "foo", true, UnsetDefault, ColumnTypeBool, UnsetSize, false}, "foo TINYINT(1)", false},
		{"MySQL small int", fields{DialectMySQL, "foo", false, UnsetDefault, ColumnTypeSmallInt, UnsetSize, true}, "foo SMALLINT NOT NULL PRIMARY KEY", false},
		{"MySQL small int nullable", fields{DialectMySQL, "foo", true, UnsetDefault, ColumnTypeSmallInt, UnsetSize, false}, "foo SMALLINT", false},
		{"MySQL int", fields{DialectMySQL, "foo", false, UnsetDefault, ColumnTypeInteger, UnsetSize, false}, "foo INT NOT NULL", false},
		{"MySQL int nullable", fields{DialectMySQL, "foo", true, UnsetDefault, ColumnTypeInteger, UnsetSize, false}, "foo INT", false},
		{"MySQL char", fields{DialectMySQL, "foo", false, UnsetDefault, ColumnTypeChar, UnsetSize, false}, "foo CHAR NOT NULL", false},
		{"MySQL char nullable", fields{DialectMySQL, "foo", true, UnsetDefault, ColumnTypeChar, UnsetSize, false}, "foo CHAR", false},
		{"MySQL varchar", fields{DialectMySQL, "foo", false, UnsetDefault, ColumnTypeVarChar, UnsetSize, false}, "foo VARCHAR NOT NULL", false},
		{"MySQL varchar nullable", fields{DialectMySQL, "foo", true, UnsetDefault, ColumnTypeVarChar, UnsetSize, false}, "foo VARCHAR", false},
		{"MySQL text", fields{DialectMySQL, "foo", false, UnsetDefault, ColumnTypeText, UnsetSize, false}, "foo TEXT NOT NULL", false},
		{"MySQL text nullable", fields{DialectMySQL, "foo", true, UnsetDefault, ColumnTypeText, UnsetSize, false}, "foo TEXT", false},
		{"MySQL datetime", fields{DialectMySQL, "foo", false, UnsetDefault, ColumnTypeDateTime, UnsetSize, false}, "foo DATETIME NOT NULL", false},
		{"MySQL datetime nullable", fields{DialectMySQL, "foo", true, UnsetDefault, ColumnTypeDateTime, UnsetSize, false}, "foo DATETIME", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Column{
				Dialect:    tt.fields.Dialect,
				Name:       tt.fields.Name,
				Nullable:   tt.fields.Nullable,
				Default:    tt.fields.Default,
				Type:       tt.fields.Type,
				Size:       tt.fields.Size,
				PrimaryKey: tt.fields.PrimaryKey,
			}
			if got, err := c.String(); got != tt.want {
				if (err != nil) != tt.wantErr {
					t.Errorf("String() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("String() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestCreateTableSqlBuilder_ToSQL(t *testing.T) {
	sql, err := DialectMySQL.
		Table("foo").
		SetIfNotExists(true).
		Column(DialectMySQL.Column("bar", ColumnTypeInteger, UnsetSize).SetPrimaryKey(true)).
		Column(DialectMySQL.Column("baz", ColumnTypeText, UnsetSize)).
		Column(DialectMySQL.Column("qux", ColumnTypeDateTime, UnsetSize).SetDefault("NOW()")).
		UniqueConstraint("bar").
		UniqueConstraint("bar", "baz").
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "CREATE TABLE IF NOT EXISTS foo ( bar INT NOT NULL PRIMARY KEY, baz TEXT NOT NULL, qux DATETIME NOT NULL DEFAULT NOW(), UNIQUE(bar), UNIQUE(bar,baz) )", sql)
}
