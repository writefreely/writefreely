package db

type RawSqlBuilder struct {
	Query string
}

func (b *RawSqlBuilder) ToSQL() (string, error) {
	return b.Query, nil
}
