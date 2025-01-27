package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ziliscite/purplelight/internal/data"
	"time"
)

type TokenRepository struct {
	db     *pgxpool.Pool
	logger *dbLogger
}

func NewTokenRepository(db *pgxpool.Pool, logger *dbLogger) TokenRepository {
	return TokenRepository{
		db:     db,
		logger: logger,
	}
}

// New The method is a shortcut which creates a new Token struct and then inserts the
// data in the tokens table.
func (t TokenRepository) New(userID int64, ttl time.Duration, scope string) (*data.Token, error) {
	token, err := data.GenerateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = t.Insert(token)
	if err != nil {
		return nil, t.logger.handleError(err)
	}

	return token, nil
}

// Insert adds the data for a specific token to the tokens table.
func (t TokenRepository) Insert(token *data.Token) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
        INSERT INTO tokens (hash, user_id, expiry, scope) 
        VALUES ($1, $2, $3, $4)
	`

	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}

	_, err := t.db.Exec(ctx, query, args...)
	if err != nil {
		return t.logger.handleError(err)
	}

	return nil
}

// DeleteAllForUser deletes all tokens for a specific user and scope.
func (t TokenRepository) DeleteAllForUser(scope string, userID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
        DELETE FROM tokens 
        WHERE scope = $1 AND user_id = $2
	`

	_, err := t.db.Exec(ctx, query, scope, userID)
	if err != nil {
		return t.logger.handleError(err)
	}

	return nil
}
