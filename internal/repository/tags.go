package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

func (a AnimeRepository) GetAllTags() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := a.db.Query(ctx, `SELECT tag.name FROM tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		if err = rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
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
func (a AnimeRepository) upsertTags(ctx context.Context, tags []string, tx pgx.Tx) ([]int32, error) {
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

func (a AnimeRepository) getAnimeTags(ctx context.Context, id int32, tx pgx.Tx) ([]string, error) {
	tags := make([]string, 0)

	rows, err := tx.Query(ctx, `SELECT t.name FROM tag t JOIN anime_tags at ON t.id = at.tag_id WHERE at.anime_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tag string
		if err = rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (a AnimeRepository) deleteAnimeTags(ctx context.Context, id int32, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DELETE FROM anime_tags WHERE anime_id = $1`, id)
	if err != nil {
		return err
	}

	return nil
}

func (a AnimeRepository) insertAnimeTags(ctx context.Context, id int32, tagsIds []int32, tx pgx.Tx) error {
	//uses a 1-second timeout (shorter than the transaction's 5-second timeout),
	//causing premature cancellations and leaving the transaction in an invalid state.
	//ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	//defer cancel()

	for _, tagId := range tagsIds {
		_, err := tx.Exec(ctx, `INSERT INTO anime_tags (anime_id, tag_id) VALUES ($1, $2)`, id, tagId)
		if err != nil {
			return err
		}
	}

	return nil
}
