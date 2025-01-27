package repository

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ziliscite/purplelight/internal/data"
	"strings"
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

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
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

	args := []interface{}{anime.Title, anime.Type, anime.Episodes, anime.Status, anime.Season, anime.Year, anime.Duration}

	err = tx.QueryRow(ctx, animeStmt.SQL, args...).
		Scan(&anime.ID, &anime.CreatedAt, &anime.Version) // value passed through a pointer
	if err != nil {
		return a.logger.handleError(err)
	}

	// Get or insert new tags
	tags, err := a.upsertTags(ctx, anime.Tags, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Insert new anime tags
	err = a.insertAnimeTags(ctx, anime.ID, tags, tx)
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `		
		SELECT
			a.id, a.title, a.type, a.episodes,
			a.status, a.season, a.year, a.duration,
			ARRAY_AGG(t.name ORDER BY t.name) AS tags,
			a.created_at, a.version
		FROM anime a
		JOIN anime_tags at ON a.id = at.anime_id
		JOIN tag t ON at.tag_id = t.id
		WHERE a.id = $1
		GROUP BY a.id, a.title, a.type, a.episodes, a.status, a.season, a.year, a.duration, a.created_at, a.version;
	`

	var anime data.Anime
	err := a.db.QueryRow(ctx, query, id).
		Scan(&anime.ID, &anime.Title, &anime.Type, &anime.Episodes, &anime.Status, &anime.Season, &anime.Year, &anime.Duration, &anime.Tags, &anime.CreatedAt, &anime.Version)
	if err != nil {
		return nil, a.logger.handleError(err)
	}

	return &anime, nil
}

func (a AnimeRepository) GetAll(title string, status string, season string, animeType string, tags []string, filters data.Filters) ([]*data.Anime, data.Metadata, error) {
	baseQuery := `
		SELECT count(*) OVER(),
			a.id, a.title, a.type, a.episodes,
			a.status, a.season, a.year, a.duration,
			ARRAY_AGG(t.name ORDER BY t.name) AS tags,
			a.created_at, a.version
		FROM anime a
		JOIN anime_tags at ON a.id = at.anime_id
		JOIN tag t ON at.tag_id = t.id
	`

	var args []interface{}
	var conditions []string

	var metadata data.Metadata

	opts := pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadOnly,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		// return an empty Metadata struct.
		return nil, metadata, a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
	}

	defer func() {
		if err != nil {
			// Rollback if an error occurs during the transaction
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				a.logger.Error(ErrTransaction.Error(), "error", rbErr)
			}
		}
	}()

	if title != "" {
		// Add wildcards in Go, use $n placeholder
		//conditions = append(conditions, fmt.Sprintf("a.title ILIKE $%d", len(args)+1))
		//args = append(args, "%"+title+"%") // Wildcard added here

		conditions = append(conditions, fmt.Sprintf(`to_tsvector('simple', a.title) @@ plainto_tsquery('simple', $%d)`, len(args)+1))
		args = append(args, title)
	}

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("a.status = $%d", len(args)+1))
		args = append(args, status)
	}

	if season != "" {
		conditions = append(conditions, fmt.Sprintf("a.season = $%d", len(args)+1))
		args = append(args, season)
	}

	if animeType != "" {
		conditions = append(conditions, fmt.Sprintf("a.type = $%d", len(args)+1))
		args = append(args, animeType)
	}

	// Combine query parts
	query := baseQuery
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if len(tags) > 0 {
		placeholders := make([]string, len(tags))
		for i := range tags {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
		}

		query = fmt.Sprintf(`
			WITH valid_anime AS (
			SELECT at.anime_id
			FROM anime_tags at
			JOIN tag t ON at.tag_id = t.id
			WHERE t.name IN (%s)
			GROUP BY at.anime_id
			HAVING COUNT(DISTINCT t.name) = %d
		)`, strings.Join(placeholders, ", "), len(tags)) + query

		for _, t := range tags {
			args = append(args, strings.Title(t))
		}

		// could just do normal concat, but this way is prettier
		query += fmt.Sprintf(" AND a.id IN (SELECT v.anime_id FROM valid_anime v)")
	}

	query += fmt.Sprintf(" GROUP BY a.id, a.title, a.type, a.episodes, a.status, a.season, a.year, a.duration, a.created_at, a.version")

	// Add an ORDER BY clause and interpolate the sort column and direction. Importantly
	// notice that we also include a secondary sort on the movie ID to ensure a consistent ordering.
	query += fmt.Sprintf(" ORDER BY a.%s %s, a.id", filters.SortColumn(), filters.SortDirection())

	// Update the SQL query to include the LIMIT and OFFSET clauses with placeholder
	// parameter values.
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d;", len(args)+1, len(args)+2)
	args = append(args, filters.Limit(), filters.Offset())

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, metadata, a.logger.handleError(err)
	}
	defer rows.Close()

	records := 0
	anime := make([]*data.Anime, 0)
	for rows.Next() {
		var an data.Anime
		if err = rows.Scan(
			&records, // Scan the count from the window function into records.
			&an.ID, &an.Title, &an.Type, &an.Episodes,
			&an.Status, &an.Season, &an.Year, &an.Duration,
			&an.Tags, &an.CreatedAt, &an.Version,
		); err != nil {
			return nil, metadata, a.logger.handleError(err)
		}

		anime = append(anime, &an)
	}

	// Generate a Metadata struct, passing in the total record count and pagination
	// parameters from the client.
	metadata.CalculateMetadata(records, filters.Page, filters.PageSize)

	if err = tx.Commit(ctx); err != nil {
		return nil, metadata, a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
	}

	// Include the metadata struct when returning.
	return anime, metadata, nil
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
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
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
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrQueryPrepare, err.Error()))
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
	err = a.deleteAnimeTags(ctx, anime.ID, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Get or insert new tags
	tags, err := a.upsertTags(ctx, anime.Tags, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Insert new anime tags
	err = a.insertAnimeTags(ctx, anime.ID, tags, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
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
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
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
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrRecordNotFound, "no rows affected"))
	}

	err = a.deleteAnimeTags(ctx, id, tx)
	if err != nil {
		return a.logger.handleError(err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return a.logger.handleError(fmt.Errorf("%w: %s", ErrTransaction, err.Error()))
	}

	return nil
}

// I'll just gonna put this here
/*
-- for tags > 0
WITH valid_anime AS (
    SELECT at.anime_id
    FROM anime_tags at
    JOIN tag t ON at.tag_id = t.id
    WHERE t.name IN ('Action', 'Isekai')
    GROUP BY at.anime_id
    HAVING COUNT(DISTINCT t.name) = 2
)
SELECT
    a.id, a.title, a.type, a.episodes,
    a.status, a.season, a.year, a.duration,
    ARRAY_AGG(t.name ORDER BY t.name) AS tags,
    a.created_at, a.version
FROM anime a
JOIN anime_tags at ON a.id = at.anime_id
JOIN tag t ON at.tag_id = t.id
WHERE a.title ILIKE '%Tanya%' AND a.type = 'OVA' AND a.id IN (SELECT v.anime_id FROM valid_anime v)
GROUP BY a.id, a.title, a.type, a.episodes, a.status, a.season, a.year, a.duration, a.created_at, a.version;

SELECT
    a.id, a.title, a.type, a.episodes,
    a.status, a.season, a.year, a.duration,
    ARRAY_AGG(t.name ORDER BY t.name) AS tags,
    a.created_at, a.version
FROM anime a
JOIN anime_tags at ON a.id = at.anime_id
JOIN tag t ON at.tag_id = t.id
WHERE 1=1
GROUP BY a.id, a.title, a.type, a.episodes, a.status, a.season, a.year, a.duration, a.created_at, a.version;

-- without
SELECT
    a.id, a.title, a.type, a.episodes,
    a.status, a.season, a.year, a.duration,
    ARRAY_AGG(t.name ORDER BY t.name) AS tags,
    a.created_at, a.version
FROM anime a
JOIN anime_tags at ON a.id = at.anime_id
JOIN tag t ON at.tag_id = t.id
WHERE to_tsvector('simple', a.title) @@ to_tsquery('simple', 'Fullmetal | Tanya') AND a.type = 'TV'
GROUP BY a.id, a.title, a.type, a.episodes, a.status, a.season, a.year, a.duration, a.created_at, a.version;

-- could also use this for AND
(to_tsvector('simple', title) @@ plainto_tsquery('simple', $1)
*/
