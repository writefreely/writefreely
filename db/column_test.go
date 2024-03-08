package db

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestColumnType_Name(t *testing.T) {
	tests := []struct {
		name    string
		ty      ColumnType
		d       DialectType
		want    string
		wantErr bool
	}{
		{"SQLite bool", ColumnTypeBool{}, DialectSQLite, "INTEGER", false},
		{"SQLite int", ColumnTypeInt{}, DialectSQLite, "INTEGER", false},
		{"SQLite string ", ColumnTypeString{HasDefault: true, DefaultVal: "that's a default"}, DialectSQLite, "TEXT DEFAULT 'that''s a default'", false},
		{"SQLite datetime", ColumnTypeDateTime{}, DialectSQLite, "DATETIME", false},

		{"MySQL bool", ColumnTypeBool{}, DialectMySQL, "BOOL", false},
		{"MySQL tiny int", ColumnTypeInt{MaxBytes: 1}, DialectMySQL, "TINYINT", false},
		{"MySQL tiny int with digits", ColumnTypeInt{MaxBytes: 1, MaxDigits: 2}, DialectMySQL, "TINYINT(2)", false},
		{"MySQL small int", ColumnTypeInt{MaxBytes: 2}, DialectMySQL, "SMALLINT", false},
		{"MySQL small int with digits", ColumnTypeInt{MaxBytes: 2, MaxDigits: 3}, DialectMySQL, "SMALLINT(3)", false},
		{"MySQL medium int", ColumnTypeInt{MaxBytes: 3}, DialectMySQL, "MEDIUMINT", false},
		{"MySQL medium int with digits", ColumnTypeInt{MaxBytes: 3, MaxDigits: 6}, DialectMySQL, "MEDIUMINT(6)", false},
		{"MySQL int", ColumnTypeInt{MaxBytes: 4}, DialectMySQL, "INTEGER", false},
		{"MySQL int with digits", ColumnTypeInt{MaxBytes: 4, MaxDigits: 11}, DialectMySQL, "INTEGER(11)", false},
		{"MySQL bigint", ColumnTypeInt{MaxBytes: 4}, DialectMySQL, "BIGINT", false},
		{"MySQL bigint with digits", ColumnTypeInt{MaxBytes: 4, MaxDigits: 15}, DialectMySQL, "BIGINT(15)", false},
		{"MySQL char", ColumnTypeString{IsFixedLength: true}, DialectMySQL, "CHAR", false},
		{"MySQL char with length", ColumnTypeString{IsFixedLength: true, MaxChars: 4}, DialectMySQL, "CHAR(4)", false},
		{"MySQL varchar with length", ColumnTypeString{MaxChars: 25}, DialectMySQL, "VARCHAR(25)", false},
		{"MySQL text", ColumnTypeString{}, DialectMySQL, "TEXT", false},
		{"MySQL datetime", ColumnTypeDateTime{}, DialectMySQL, "DATETIME", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ty.Name(tt.d)
			if (err != nil) != tt.wantErr {
				t.Errorf("Name() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Name() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColumnType_Default(t *testing.T) {
	tests := []struct {
		name    string
		ty      ColumnType
		d       DialectType
		want    string
		wantErr bool
	}{
		{"SQLite bool none", ColumnTypeBool{}, DialectSQLite, "", false},
		{"SQLite bool false", ColumnTypeBool{}, DialectSQLite, "0", false},
		{"SQLite bool true", ColumnTypeBool{}, DialectSQLite, "1", false},
		{"SQLite int none", ColumnTypeInt{}, DialectSQLite, "", false},
		{"SQLite int empty", ColumnTypeInt{HasDefault: true}, DialectSQLite, "0", false},
		{"SQLite int", ColumnTypeInt{HasDefault: true, DefaultVal: 10}, DialectSQLite, "10", false},
		{"SQLite string none", ColumnTypeString{}, DialectSQLite, "", false},
		{"SQLite string empty", ColumnTypeString{HasDefault: true}, DialectSQLite, "''", false},
		{"SQLite string", ColumnTypeString{HasDefault: true, DefaultVal: "that's a default"}, DialectSQLite, "'that''s a default'", false},
		{"MySQL string", ColumnTypeString{HasDefault: true, DefaultVal: "%that's a default%"}, DialectMySQL, "'%that\\'s a default%'", false},

		{"SQLite datetime none", ColumnTypeDateTime{}, DialectSQLite, "", false},
		{"SQLite datetime now", ColumnTypeDateTime{DefaultVal: DefaultNow}, DialectSQLite, "CURRENT_TIMESTAMP", false},
		{"MySQL datetime now", ColumnTypeDateTime{DefaultVal: DefaultNow}, DialectMySQL, "NOW()", false},
		{"PostgreSQL datetime now", ColumnTypeDateTime{DefaultVal: DefaultNow}, DialectPostgreSQL, "CURRENT_TIMESTAMP", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ty.Default(tt.d)
			if (err != nil) != tt.wantErr {
				t.Errorf("Default() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Default() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColumn_CreateSQL(t *testing.T) {
	type fields struct {
		Dialect    DialectType
		Name       string
		Nullable   bool
		Type       ColumnType
		PrimaryKey bool
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{"SQLite bool", fields{DialectSQLite, "foo", false, ColumnTypeBool{}, false}, "foo INTEGER NOT NULL", false},
		{"SQLite bool nullable", fields{DialectSQLite, "foo", true, ColumnTypeBool{}, false}, "foo INTEGER", false},
		{"SQLite int", fields{DialectSQLite, "foo", false, ColumnTypeInt{}, true}, "foo INTEGER NOT NULL PRIMARY KEY", false},
		{"SQLite int nullable", fields{DialectSQLite, "foo", true, ColumnTypeInt{}, false}, "foo INTEGER", false},
		{"SQLite text", fields{DialectSQLite, "foo", false, ColumnTypeString{}, false}, "foo TEXT NOT NULL", false},
		{"SQLite text nullable", fields{DialectSQLite, "foo", true, ColumnTypeString{}, false}, "foo TEXT", false},
		{"SQLite datetime", fields{DialectSQLite, "foo", false, ColumnTypeDateTime{}, false}, "foo DATETIME NOT NULL", false},
		{"SQLite datetime nullable", fields{DialectSQLite, "foo", true, ColumnTypeDateTime{}, false}, "foo DATETIME", false},

		{"MySQL bool", fields{DialectMySQL, "foo", false, ColumnTypeBool{}, false}, "foo TINYINT(1) NOT NULL", false},
		{"MySQL bool nullable", fields{DialectMySQL, "foo", true, ColumnTypeBool{}, false}, "foo TINYINT(1)", false},
		{"MySQL tiny int", fields{DialectMySQL, "foo", false, ColumnTypeInt{MaxBytes: 1}, true}, "foo TINYINT NOT NULL PRIMARY KEY", false},
		{"MySQL tiny int nullable", fields{DialectMySQL, "foo", true, ColumnTypeInt{MaxBytes: 1}, false}, "foo TINYINT", false},
		{"MySQL small int", fields{DialectMySQL, "foo", false, ColumnTypeInt{MaxBytes: 2}, true}, "foo SMALLINT NOT NULL PRIMARY KEY", false},
		{"MySQL small int nullable", fields{DialectMySQL, "foo", true, ColumnTypeInt{MaxBytes: 2}, false}, "foo SMALLINT", false},
		{"MySQL int", fields{DialectMySQL, "foo", false, ColumnTypeInt{MaxBytes: 4}, true}, "foo INTEGER NOT NULL PRIMARY KEY", false},
		{"MySQL int nullable", fields{DialectMySQL, "foo", true, ColumnTypeInt{MaxBytes: 4}, false}, "foo INTEGER", false},
		{"MySQL big int", fields{DialectMySQL, "foo", false, ColumnTypeInt{}, true}, "foo BIGINT NOT NULL PRIMARY KEY", false},
		{"MySQL big int nullable", fields{DialectMySQL, "foo", true, ColumnTypeInt{}, false}, "foo BIGINT", false},
		{"MySQL char", fields{DialectMySQL, "foo", false, ColumnTypeString{IsFixedLength: true}, false}, "foo CHAR NOT NULL", false},
		{"MySQL char nullable", fields{DialectMySQL, "foo", true, ColumnTypeString{IsFixedLength: true}, false}, "foo CHAR", false},
		{"MySQL varchar", fields{DialectMySQL, "foo", false, ColumnTypeString{MaxChars: 255}, false}, "foo VARCHAR(255) NOT NULL", false},
		{"MySQL varchar nullable", fields{DialectMySQL, "foo", true, ColumnTypeString{MaxChars: 255}, false}, "foo VARCHAR(255)", false},
		{"MySQL text", fields{DialectMySQL, "foo", false, ColumnTypeString{}, false}, "foo TEXT NOT NULL", false},
		{"MySQL text nullable", fields{DialectMySQL, "foo", true, ColumnTypeString{}, false}, "foo TEXT", false},
		{"MySQL datetime", fields{DialectMySQL, "foo", false, ColumnTypeDateTime{}, false}, "foo DATETIME NOT NULL", false},
		{"MySQL datetime nullable", fields{DialectMySQL, "foo", true, ColumnTypeDateTime{}, false}, "foo DATETIME", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Column{
				Name:       tt.fields.Name,
				Nullable:   tt.fields.Nullable,
				Type:       tt.fields.Type,
				PrimaryKey: tt.fields.PrimaryKey,
			}
			if got, err := c.CreateSQL(tt.fields.Dialect); got != tt.want {
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
