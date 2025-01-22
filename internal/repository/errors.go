package repository

import (
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrTooManyRows          = errors.New("too many rows returned")
	ErrRecordNotFound       = errors.New("record not found")
	ErrDuplicateEntry       = errors.New("duplicate entry")
	ErrForeignKeyViolation  = errors.New("foreign key violation")
	ErrNotNullViolation     = errors.New("null value not allowed")
	ErrStringDataTruncation = errors.New("value too long for column")
	ErrSyntaxError          = errors.New("syntax error in SQL statement")
	ErrSerializationFailure = errors.New("transaction serialization failure")
	ErrFeatureNotSupported  = errors.New("SQL feature not supported")
	ErrDeadlockDetected     = errors.New("deadlock detected")
	ErrPrivilegeViolation   = errors.New("privilege violation")
	ErrDataTypeMismatch     = errors.New("data type mismatch")
	ErrConnectionFailure    = errors.New("database connection failure")
	ErrReadOnlyDatabase     = errors.New("database is in read-only mode")
	ErrFailedCloseRows      = errors.New("failed to close rows")
	ErrDatabaseUnknown      = errors.New("unknown database error")
	ErrFailedCloseStmt      = errors.New("failed to close stmt")
	ErrTransaction          = errors.New("transaction failed")
	ErrQueryPrepare         = errors.New("failed preparing query")
	ErrInternalDatabase     = errors.New("internal database error")
)

// handleError will handle potential database execution errors, returning a generic error and message.
func (l *dbLogger) handleError(err error) error {
	var pgErr *pgconn.PgError
	// check for postgresql specific errors
	if errors.As(err, &pgErr) {
		l.Error(ErrDatabaseUnknown.Error(), "error", pgErr.Message)

		// Return corresponding error code
		switch pgErr.Code {
		case "23505": // Unique constraint violation
			return ErrDuplicateEntry
		case "42P05":
			return ErrDuplicateEntry
		case "23503": // Foreign key violation
			return ErrForeignKeyViolation
		case "23502": // Not-null violation
			return ErrNotNullViolation
		case "22001": // String data truncation
			return ErrStringDataTruncation
		case "42601": // Syntax error
			return ErrSyntaxError
		case "40001": // Serialization failure
			return ErrSerializationFailure
		case "0A000": // Feature is not supported
			return ErrFeatureNotSupported
		case "40P01": // Deadlock detected
			return ErrDeadlockDetected
		case "42501": // Privilege violation
			return ErrPrivilegeViolation
		case "42883": // Data type mismatch
			return ErrDataTypeMismatch
		case "08006": // Connection failure
			return ErrConnectionFailure
		case "25006": // Database is in read-only mode
			return ErrReadOnlyDatabase
		default:
			return ErrDatabaseUnknown
		}
	}

	// Log the generic database error
	l.Error(ErrInternalDatabase.Error(), "error", err.Error())

	// check for database generic errors
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrRecordNotFound
	case errors.Is(err, pgx.ErrTxClosed):
		return ErrTransaction
	case errors.Is(err, pgx.ErrTooManyRows):
		return ErrTooManyRows
	case errors.Is(err, ErrFailedCloseStmt):
		return ErrFailedCloseStmt
	case errors.Is(err, ErrFailedCloseRows):
		return ErrFailedCloseRows
	case errors.Is(err, ErrQueryPrepare):
		return ErrQueryPrepare
	case errors.Is(err, ErrTransaction):
		return ErrTransaction
	default:
		return ErrInternalDatabase
	}
}
