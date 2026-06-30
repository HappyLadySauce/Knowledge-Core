package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
)

// IsUniqueViolation reports whether err is a PostgreSQL unique constraint violation.
// IsUniqueViolation 判断 err 是否为 PostgreSQL 唯一约束冲突。
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// IsForeignKeyViolation reports whether err is a PostgreSQL foreign key violation.
// IsForeignKeyViolation 判断 err 是否为 PostgreSQL 外键约束冲突。
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation
}

// IsConstraintViolation reports whether err is a known PostgreSQL constraint violation.
// IsConstraintViolation 判断 err 是否为已知 PostgreSQL 约束冲突。
func IsConstraintViolation(err error) bool {
	return IsUniqueViolation(err) || IsForeignKeyViolation(err)
}
