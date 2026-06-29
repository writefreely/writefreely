package db

import "testing"

func TestDialectType_Table_SQLite(t *testing.T) {
	b := DialectSQLite.Table("users")
	if b == nil {
		t.Fatal("Table() returned nil")
	}
	if b.Dialect != DialectSQLite {
		t.Errorf("Table().Dialect = %v, want DialectSQLite", b.Dialect)
	}
	if b.Name != "users" {
		t.Errorf("Table().Name = %q, want %q", b.Name, "users")
	}
}

func TestDialectType_Table_MySQL(t *testing.T) {
	b := DialectMySQL.Table("users")
	if b == nil {
		t.Fatal("Table() returned nil")
	}
	if b.Dialect != DialectMySQL {
		t.Errorf("Table().Dialect = %v, want DialectMySQL", b.Dialect)
	}
}

func TestDialectType_AlterTable_SQLite(t *testing.T) {
	b := DialectSQLite.AlterTable("posts")
	if b == nil {
		t.Fatal("AlterTable() returned nil")
	}
	if b.Dialect != DialectSQLite {
		t.Errorf("AlterTable().Dialect = %v, want DialectSQLite", b.Dialect)
	}
	if b.Name != "posts" {
		t.Errorf("AlterTable().Name = %q, want %q", b.Name, "posts")
	}
}

func TestDialectType_AlterTable_MySQL(t *testing.T) {
	b := DialectMySQL.AlterTable("posts")
	if b == nil {
		t.Fatal("AlterTable() returned nil")
	}
	if b.Dialect != DialectMySQL {
		t.Errorf("AlterTable().Dialect = %v, want DialectMySQL", b.Dialect)
	}
}

func TestDialect_ColumnDialectField(t *testing.T) {
	tests := []struct {
		name        string
		dialect     DialectType
		colName     string
		colType     ColumnType
		wantDialect DialectType
	}{
		{
			name:        "SQLite column preserves dialect",
			dialect:     DialectSQLite,
			colName:     "id",
			colType:     ColumnTypeInteger,
			wantDialect: DialectSQLite,
		},
		{
			name:        "MySQL column preserves dialect",
			dialect:     DialectMySQL,
			colName:     "username",
			colType:     ColumnTypeVarChar,
			wantDialect: DialectMySQL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := tt.dialect.Column(tt.colName, tt.colType, UnsetSize)
			if col == nil {
				t.Fatal("Column() returned nil")
			}
			if col.Dialect != tt.wantDialect {
				t.Errorf("Column().Dialect = %v, want %v", col.Dialect, tt.wantDialect)
			}
			if col.Name != tt.colName {
				t.Errorf("Column().Name = %q, want %q", col.Name, tt.colName)
			}
			if col.Type != tt.colType {
				t.Errorf("Column().Type = %v, want %v", col.Type, tt.colType)
			}
		})
	}
}

func TestDialect_CreateIndex_PreservesDialect(t *testing.T) {
	sqliteIdx := DialectSQLite.CreateIndex("idx_test", "posts", "user_id")
	if sqliteIdx.Dialect != DialectSQLite {
		t.Errorf("CreateIndex dialect = %v, want DialectSQLite", sqliteIdx.Dialect)
	}

	mysqlIdx := DialectMySQL.CreateUniqueIndex("uq_test", "users", "email")
	if mysqlIdx.Dialect != DialectMySQL {
		t.Errorf("CreateUniqueIndex dialect = %v, want DialectMySQL", mysqlIdx.Dialect)
	}
}

func TestDialect_DropIndex_PreservesDialect(t *testing.T) {
	sqliteIdx := DialectSQLite.DropIndex("idx_test", "posts")
	if sqliteIdx.Dialect != DialectSQLite {
		t.Errorf("DropIndex dialect = %v, want DialectSQLite", sqliteIdx.Dialect)
	}

	mysqlIdx := DialectMySQL.DropIndex("idx_test", "posts")
	if mysqlIdx.Dialect != DialectMySQL {
		t.Errorf("DropIndex dialect = %v, want DialectMySQL", mysqlIdx.Dialect)
	}
}
