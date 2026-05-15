package store

import "database/sql"

type Dialect interface {
	DriverName() string
	CreateTablesSQL() []string
	MigrationsSQL() []string
	BoolFalse() string
	ResetSequenceOnEmpty(db *sql.DB, table string)
	ImportWipe(tx *sql.Tx)
	ImportResetSequences(tx *sql.Tx)
}

// rewritePlaceholders converts ? markers to $1, $2, etc. for Postgres.
// For SQLite (or any dialect not needing rewrite), returns the input unchanged.
func rewritePlaceholders(query string, dollarStyle bool) string {
	if !dollarStyle {
		return query
	}
	buf := make([]byte, 0, len(query)+32)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			buf = append(buf, '$')
			if n >= 10 {
				buf = append(buf, byte('0'+n/10))
			}
			buf = append(buf, byte('0'+n%10))
		} else {
			buf = append(buf, query[i])
		}
	}
	return string(buf)
}
