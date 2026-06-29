package db

import "testing"

func TestRawSqlBuilder_ToSQL(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "simple query returned unchanged",
			query: "SELECT 1",
		},
		{
			name:  "complex query",
			query: "ALTER TABLE posts ADD COLUMN views INT NOT NULL DEFAULT 0",
		},
		{
			name:  "empty query",
			query: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &RawSqlBuilder{Query: tt.query}
			got, err := b.ToSQL()
			if err != nil {
				t.Fatalf("ToSQL() unexpected error: %v", err)
			}
			if got != tt.query {
				t.Errorf("ToSQL() = %q, want %q", got, tt.query)
			}
		})
	}
}
