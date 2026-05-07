package db

import "testing"

func TestCreateIndexSqlBuilder_ToSQL(t *testing.T) {
	tests := []struct {
		name    string
		builder *CreateIndexSqlBuilder
		want    string
		wantErr bool
	}{
		{
			name:    "single column non-unique",
			builder: DialectMySQL.CreateIndex("idx_posts_user", "posts", "user_id"),
			want:    "CREATE INDEX idx_posts_user on posts (user_id)",
			wantErr: false,
		},
		{
			name:    "single column unique",
			builder: DialectMySQL.CreateUniqueIndex("uq_users_email", "users", "email"),
			want:    "CREATE UNIQUE INDEX uq_users_email on users (email)",
			wantErr: false,
		},
		{
			name:    "multi-column index",
			builder: DialectSQLite.CreateIndex("idx_posts_col", "posts", "collection_id", "created"),
			want:    "CREATE INDEX idx_posts_col on posts (collection_id, created)",
			wantErr: false,
		},
		{
			name:    "multi-column unique index",
			builder: DialectSQLite.CreateUniqueIndex("uq_multi", "table1", "col_a", "col_b", "col_c"),
			want:    "CREATE UNIQUE INDEX uq_multi on table1 (col_a, col_b, col_c)",
			wantErr: false,
		},
		{
			name:    "no columns returns error",
			builder: &CreateIndexSqlBuilder{Dialect: DialectMySQL, Name: "idx_empty", Table: "posts"},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.builder.ToSQL()
			if (err != nil) != tt.wantErr {
				t.Errorf("ToSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToSQL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDropIndexSqlBuilder_ToSQL(t *testing.T) {
	tests := []struct {
		name    string
		builder *DropIndexSqlBuilder
		want    string
	}{
		{
			name:    "MySQL drop index",
			builder: DialectMySQL.DropIndex("idx_posts_user", "posts"),
			want:    "DROP INDEX idx_posts_user on posts",
		},
		{
			name:    "SQLite drop index",
			builder: DialectSQLite.DropIndex("idx_col", "table1"),
			want:    "DROP INDEX idx_col on table1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.builder.ToSQL()
			if err != nil {
				t.Fatalf("ToSQL() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ToSQL() = %q, want %q", got, tt.want)
			}
		})
	}
}
