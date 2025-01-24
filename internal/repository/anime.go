package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ziliscite/purplelight/internal/data"
	"time"
)

// AnimeRepository Define a AnimeRepository struct type which wraps a sql.DB connection pool.
type AnimeRepository struct {
	db     *pgxpool.Pool
	logger *dbLogger
}

func NewAnimeRepository(db *pgxpool.Pool, logger *dbLogger) AnimeRepository {
	return AnimeRepository{
		db:     db,
		logger: logger,
	}
}

// InsertAnime Add a placeholder method for inserting a new record in the movies table.
func (a AnimeRepository) InsertAnime(anime *data.Anime) error {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted, // Set isolation level
		AccessMode: pgx.ReadWrite,     // Specify read-write mode
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

	// Insert anime through the main transaction
	animeStmt, err := tx.Prepare(ctx, "insert anime", `
		INSERT INTO anime (title, type, episodes, status, season, year, duration)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, version
	`)
	if err != nil {
		a.logger.Error(ErrQueryPrepare.Error(), "error", err)
		return ErrQueryPrepare
	}

	err = tx.QueryRow(ctx, animeStmt.SQL, anime.Title, anime.Type, anime.Episodes, anime.Status, anime.Season, anime.Year, anime.Duration).
		Scan(&anime.ID, &anime.CreatedAt, &anime.Version) // value passed through a pointer
	if err != nil {
		return a.logger.handleError(err)
	}

	// Get or insert new anime tags
	tags, err := a.upsertTags(anime.Tags, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	err = a.insertAnimeTags(anime.ID, tags, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
	}

	return nil
}

// GetAnime Add a placeholder method for fetching a specific record from the movies table.
func (a AnimeRepository) GetAnime(id int32) (*data.Anime, error) {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadOnly,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
	}

	defer func() {
		if err != nil {
			// Rollback if an error occurs during the transaction
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				a.logger.Error(ErrTransaction.Error(), "error", rbErr)
			}
		}
	}()

	animeStmt, err := tx.Prepare(ctx, "get anime", `
		SELECT * FROM anime WHERE id = $1
	`)
	if err != nil {
		return nil, a.logger.handleError(fmt.Errorf("%w: %s", ErrQueryPrepare, err.Error()))

	}

	var anime data.Anime
	err = tx.QueryRow(ctx, animeStmt.SQL, id).
		Scan(&anime.ID, &anime.Title, &anime.Type, &anime.Episodes, &anime.Status, &anime.Season, &anime.Year, &anime.Duration, &anime.CreatedAt, &anime.Version)
	if err != nil {
		return nil, a.logger.handleError(err)
	}

	tags, err := a.getAnimeTags(id, tx)
	if err != nil {
		return nil, a.logger.handleError(err)
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
	opts := pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return nil
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				a.logger.Error(ErrTransaction.Error(), "error", rbErr)
			}
		}
	}()

	// Add the 'AND version = $6' clause to the SQL query
	animeStmt, err := tx.Prepare(ctx, "update anime", `
		UPDATE anime 
		SET title = $1, type = $2, episodes = $3, 
		    status = $4, season = $5, year = $6, 
		    duration = $7, version = version + 1
		WHERE id = $8 AND version = $9
		RETURNING version
	`)
	if err != nil {
		a.logger.Error(ErrQueryPrepare.Error(), "error", err)
		return ErrQueryPrepare
	}

	// Update anime record
	// Execute the SQL query. If no matching row could be found, we know the movie
	// version has changed (or the record has been deleted) and we return our custom
	// ErrEditConflict error.
	err = tx.QueryRow(ctx,
		animeStmt.SQL, anime.Title, anime.Type, anime.Episodes, anime.Status,
		anime.Season, anime.Year, anime.Duration, anime.ID, anime.Version,
	).
		Scan(&anime.Version)
	if err != nil {
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrEditConflict, err.Error()))
	}

	// Delete current anime tags
	err = a.deleteAnimeTags(anime.ID, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Get or insert new tags
	tags, err := a.upsertTags(anime.Tags, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Insert new anime tags
	err = a.insertAnimeTags(anime.ID, tags, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return ErrTransaction
	}

	return nil
}

// DeleteAnime Add a placeholder method for deleting a specific record from the movies table.
func (a AnimeRepository) DeleteAnime(id int32) error {
	// Return an ErrRecordNotFound error if the movie ID is less than 1.
	if id < 1 {
		a.logger.Error(ErrRecordNotFound.Error(), "error", "id must be greater than 0")
		return ErrRecordNotFound
	}

	opts := pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return nil
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				a.logger.Error(ErrTransaction.Error(), "error", rbErr)
			}
		}
	}()

	// Execute the SQL query using the Exec() method, passing in the id variable as
	// the value for the placeholder parameter. The Exec() method returns a sql.Result
	res, err := tx.Exec(ctx, `DELETE FROM anime WHERE id = $1`, id)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Call the RowsAffected() method on the sql.Result object to get the number of rows
	// affected by the query.
	rowsAffected := res.RowsAffected()

	// If no rows were affected, we know that the movies table didn't contain a record
	// with the provided ID at the moment we tried to delete it. In that case we
	// return an ErrRecordNotFound error.
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	err = a.deleteAnimeTags(id, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		a.logger.Error(ErrTransaction.Error(), "error", err)
		return ErrTransaction
	}

	return nil
}

// upsertTag will get or insert a tag by name, returning the tag id.
func (a AnimeRepository) upsertTag(tag string, tx pgx.Tx) (int32, error) {
	var tagId int32

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := tx.QueryRow(ctx, `INSERT INTO tag (name)
		VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name=excluded.name
		RETURNING id`, tag).Scan(&tagId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	return tagId, nil
}

// upsertTags will bulk upsert tags by name, returning the tag ids.
// for this, use these options pgx.ReadCommitted, // or pgx.RepeatableRead
// ReadCommitted: Ensures that transactions only see committed data, but allows for some level of concurrency.
// RepeatableRead: Ensures that if a transaction reads a row, it will see the same data for the entire duration of the transaction,
// but can still allow for some changes in data as long as it doesn't conflict with other transactions.
func (a AnimeRepository) upsertTags(tags []string, tx pgx.Tx) ([]int32, error) {
	var tagIds []int32

	batch := &pgx.Batch{}
	for _, tag := range tags {
		// Batch adding the upsert statement for each tag
		batch.Queue(`
			INSERT INTO tag (name) 
			VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name=excluded.name
			RETURNING id
		`, tag)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	br := tx.SendBatch(ctx, batch)
	defer func(br pgx.BatchResults) {
		err := br.Close()
		if err != nil {
			a.logger.Error(ErrFailedCloseRows.Error(), "error", err)
		}
	}(br)

	// Execute the batch and get the tag ids
	for i := 0; i < len(tags); i++ {
		var tagId int32
		if err := br.QueryRow().Scan(&tagId); err != nil {
			return nil, err
		}

		tagIds = append(tagIds, tagId)
	}

	return tagIds, nil
}

func (a AnimeRepository) getAnimeTags(id int32, tx pgx.Tx) ([]string, error) {
	tags := make([]string, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	rows, err := tx.Query(ctx, `SELECT t.name FROM tag t JOIN anime_tags at ON t.id = at.tag_id WHERE at.anime_id = $1`, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (a AnimeRepository) deleteAnimeTags(id int32, tx pgx.Tx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := tx.Exec(ctx, `DELETE FROM anime_tags WHERE anime_id = $1`, id)
	if err != nil {
		return err
	}

	return nil
}

func (a AnimeRepository) insertAnimeTags(id int32, tagsIds []int32, tx pgx.Tx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for _, tagId := range tagsIds {
		_, err := tx.Exec(ctx, `INSERT INTO anime_tags (anime_id, tag_id) VALUES ($1, $2)`, id, tagId)
		if err != nil {
			return err
		}
	}

	return nil
}
