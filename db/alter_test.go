package db

import "testing"

func TestAlterTableSqlBuilder_ToSQL(t *testing.T) {
	type fields struct {
		Dialect DialectType
		Name    string
		Changes []string
	}
	tests := []struct {
		name    string
		builder *AlterTableSqlBuilder
		want    string
		wantErr bool
	}{
		{
			name: "MySQL add int",
			builder: DialectMySQL.
				AlterTable("the_table").
				AddColumn(NonNullableColumn("the_col", ColumnTypeInt{MaxBytes: 4})),
			want:    "ALTER TABLE the_table ADD COLUMN the_col INTEGER NOT NULL",
			wantErr: false,
		},
		{
			name: "MySQL add string",
			builder: DialectMySQL.
				AlterTable("the_table").
				AddColumn(NonNullableColumn("the_col", ColumnTypeString{MaxChars: 128})),
			want:    "ALTER TABLE the_table ADD COLUMN the_col VARCHAR(128) NOT NULL",
			wantErr: false,
		},
		{
			name: "MySQL add int and string",
			builder: DialectMySQL.
				AlterTable("the_table").
				AddColumn(NonNullableColumn("first_col", ColumnTypeInt{MaxBytes: 4})).
				AddColumn(NonNullableColumn("second_col", ColumnTypeString{MaxChars: 128})),
			want:    "ALTER TABLE the_table ADD COLUMN first_col INT NOT NULL, ADD COLUMN second_col VARCHAR(128) NOT NULL",
			wantErr: false,
		},
		{
			name: "MySQL change to string",
			builder: DialectMySQL.
				AlterTable("the_table").
				ChangeColumn("old_col", NonNullableColumn("new_col", ColumnTypeString{})),
			want:    "ALTER TABLE the_table RENAME COLUMN old_col TO new_col, MODIFY COLUMN new_col TEXT NOT NULL",
			wantErr: false,
		},
		{
			name: "PostgreSQL change to int",
			builder: DialectMySQL.
				AlterTable("the_table").
				ChangeColumn("old_col", NullableColumn("new_col", ColumnTypeInt{MaxBytes: 4})),
			want:    "ALTER TABLE the_table RENAME COLUMN old_col TO new_col, ALTER COLUMN new_col TYPE INTEGER, ALTER COLUMN new_col DROP NOT NULL",
			wantErr: false,
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
				t.Errorf("ToSQL() got = %v, want %v", got, tt.want)
			}
		})
	}
}
