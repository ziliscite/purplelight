package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ziliscite/purplelight/internal/data"
	"log/slog"
	"runtime"
	"strings"
	"time"
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

	ErrDatabaseUnknown  = errors.New("unknown database error")
	ErrFailedCloseStmt  = errors.New("failed to close stmt")
	ErrTransaction      = errors.New("transaction failed")
	ErrQueryPrepare     = errors.New("failed preparing query")
	ErrInternalDatabase = errors.New("internal database error")
)

type dbLogger struct {
	sl *slog.Logger
}

func (l *dbLogger) Error(msg string, args ...any) {
	_, file, line, _ := runtime.Caller(1)
	shortFile := file
	if strings.Contains(file, "GolandProjects/purplelight") {
		shortFile = strings.Replace(file, "C:/Users/manzi/GolandProjects/purplelight", ".", 1)
	}
	trace := fmt.Sprintf("%s:%d", shortFile, line)
	args = append(args, "trace", trace)
	l.sl.Error(msg, args...)
}

// AnimeRepository Define a AnimeRepository struct type which wraps a sql.DB connection pool.
type AnimeRepository struct {
	db     *pgxpool.Pool
	logger *dbLogger
}

func NewAnimeRepository(db *pgxpool.Pool, logger *slog.Logger) AnimeRepository {
	return AnimeRepository{
		db:     db,
		logger: &dbLogger{logger},
	}
}

// InsertAnime Add a placeholder method for inserting a new record in the movies table.
func (a AnimeRepository) InsertAnime(anime *data.Anime) error {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.Serializable, // Set isolation level
		AccessMode: pgx.ReadWrite,    // Specify read-write mode
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return ErrTransaction
	}

	defer func() {
		if err != nil {
			// Rollback if an error occurs during the transaction
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				a.logger.Error(ErrTransaction.Error(), "error", rbErr)
			}
		}
	}()

	// Insert anime
	animeStmt, err := tx.Prepare(ctx, "anime", `
		INSERT INTO anime (title, type, episodes, status, season, year, duration)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, version
	`)
	if err != nil {
		a.logger.Error(ErrQueryPrepare.Error(), "error", err)
		return ErrQueryPrepare
	}

	err = tx.QueryRow(ctx, animeStmt.SQL, anime.Title, anime.Type, anime.Episodes, anime.Status, anime.Season, anime.Year, anime.Duration).
		Scan(&anime.ID, &anime.CreatedAt, &anime.Version)
	if err != nil {
		err, msg := a.handleError(err)
		a.logger.Error(err.Error(), "error", msg)
		return err
	}

	var tags []int64
	for _, tag := range anime.Tags {
		// get (id) or insert tag by name
		tagId, err := a.getOrInsertTag(tag, tx)
		if err != nil {
			err, msg := a.handleError(err)
			a.logger.Error(err.Error(), "error", msg)
			return err
		}

		tags = append(tags, tagId)
	}

	tagsStmt, err := tx.Prepare(ctx, "anime tags", `
		INSERT INTO anime_tags (anime_id, tag_id)
		VALUES ($1, $2)
	`)
	if err != nil {
		a.logger.Error(ErrQueryPrepare.Error(), "error", err)
		return ErrQueryPrepare
	}

	// Insert anime_tags
	for _, tagId := range tags {
		_, err := tx.Exec(ctx, tagsStmt.SQL, anime.ID, tagId)
		if err != nil {
			err, msg := a.handleError(err)
			a.logger.Error(err.Error(), "error", msg)
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return ErrTransaction
	}

	return nil
}

// GetAnime Add a placeholder method for fetching a specific record from the movies table.
func (a AnimeRepository) GetAnime(id int64) (*data.Anime, error) {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.Serializable, // Set isolation level
		AccessMode: pgx.ReadOnly,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return nil, ErrTransaction
	}

	defer func() {
		if err != nil {
			// Rollback if an error occurs during the transaction
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				a.logger.Error(ErrTransaction.Error(), "error", rbErr)
			}
		}
	}()

	animeStmt, err := tx.Prepare(ctx, "anime", `
		SELECT * FROM anime WHERE id = $1
	`)
	if err != nil {
		a.logger.Error(ErrQueryPrepare.Error(), "error", err)
		return nil, ErrQueryPrepare
	}

	var anime data.Anime
	err = tx.QueryRow(ctx, animeStmt.SQL, id).
		Scan(&anime.ID, &anime.Title, &anime.Type, &anime.Episodes, &anime.Status, &anime.Season, &anime.Year, &anime.Duration, &anime.CreatedAt, &anime.Version)
	if err != nil {
		err, msg := a.handleError(err)
		a.logger.Error(err.Error(), "error", msg)
		return nil, err
	}

	tags, err := a.getAnimeTags(id, tx)
	if err != nil {
		err, msg := a.handleError(err)
		a.logger.Error(err.Error(), "error", msg)
		return nil, err
	}

	anime.Tags = tags

	if err := tx.Commit(ctx); err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return nil, ErrTransaction
	}

	return &anime, nil
}

// UpdateAnime Add a placeholder method for updating a specific record in the movies table.
func (a AnimeRepository) UpdateAnime(anime *data.Anime) error {
	return nil
}

// DeleteAnime Add a placeholder method for deleting a specific record from the movies table.
func (a AnimeRepository) DeleteAnime(id int64) error {
	return nil
}

// getOrInsertTag will get or insert a tag by name, returning the tag id.
func (a AnimeRepository) getOrInsertTag(tag string, tx pgx.Tx) (int64, error) {
	var tagId int64

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := tx.QueryRow(ctx, `SELECT id FROM tag WHERE name = $1`, tag).Scan(&tagId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO tag (name) VALUES ($1) RETURNING id`, tag).Scan(&tagId)
		if err != nil {
			return 0, err
		}
	}

	return tagId, nil
}

func (a AnimeRepository) getAnimeTags(id int64, tx pgx.Tx) ([]string, error) {
	tags := make([]string, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	rows, err := tx.Query(ctx, `SELECT t.name FROM tag t JOIN anime_tags at ON t.id = at.tag_id WHERE at.anime_id = $1`, id)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// handleError will handle potential database execution errors, returning a generic error and message.
func (a AnimeRepository) handleError(err error) (error, string) {
	var pgErr *pgconn.PgError
	// check for postgresql specific errors
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // Unique constraint violation
			return ErrDuplicateEntry, pgErr.Message
		case "23503": // Foreign key violation
			return ErrForeignKeyViolation, pgErr.Message
		case "23502": // Not-null violation
			return ErrNotNullViolation, pgErr.Message
		case "22001": // String data truncation
			return ErrStringDataTruncation, pgErr.Message
		case "42601": // Syntax error
			return ErrSyntaxError, pgErr.Message
		case "40001": // Serialization failure
			return ErrSerializationFailure, pgErr.Message
		case "0A000": // Feature is not supported
			return ErrFeatureNotSupported, pgErr.Message
		default:
			return ErrDatabaseUnknown, pgErr.Message
		}
	}

	// check for database generic errors
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrRecordNotFound, err.Error()
	case errors.Is(err, pgx.ErrTxClosed):
		return ErrTransaction, err.Error()
	case errors.Is(err, pgx.ErrTooManyRows):
		return ErrTooManyRows, err.Error()
	default:
		return ErrInternalDatabase, err.Error()
	}
}
