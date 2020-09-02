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
				AddColumn(DialectMySQL.Column("the_col", ColumnTypeInteger, UnsetSize)),
			want:    "ALTER TABLE the_table ADD COLUMN the_col INT NOT NULL",
			wantErr: false,
		},
		{
			name: "MySQL add string",
			builder: DialectMySQL.
				AlterTable("the_table").
				AddColumn(DialectMySQL.Column("the_col", ColumnTypeVarChar, OptionalInt{true, 128})),
			want:    "ALTER TABLE the_table ADD COLUMN the_col VARCHAR(128) NOT NULL",
			wantErr: false,
		},

		{
			name: "MySQL add int and string",
			builder: DialectMySQL.
				AlterTable("the_table").
				AddColumn(DialectMySQL.Column("first_col", ColumnTypeInteger, UnsetSize)).
				AddColumn(DialectMySQL.Column("second_col", ColumnTypeVarChar, OptionalInt{true, 128})),
			want:    "ALTER TABLE the_table ADD COLUMN first_col INT NOT NULL, ADD COLUMN second_col VARCHAR(128) NOT NULL",
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
