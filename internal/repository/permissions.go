package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ziliscite/purplelight/internal/data"
	"time"
)

type PermissionRepository struct {
	db     *pgxpool.Pool
	logger *dbLogger
}

func NewPermissionRepository(db *pgxpool.Pool, logger *dbLogger) PermissionRepository {
	return PermissionRepository{
		db:     db,
		logger: logger,
	}
}

// GetAllForUser method returns all permission codes for a specific user in a
// Permissions slice. The code in this method should feel very familiar --- it uses the
// standard pattern that we've already seen before for retrieving multiple data rows in
// an SQL query.
func (p PermissionRepository) GetAllForUser(userID int64) (data.Permissions, error) {
	query := `
        SELECT p.code
        FROM permissions p
        INNER JOIN users_permissions up ON up.permission_id = p.id
        INNER JOIN users u ON up.user_id = u.id
        WHERE u.id = $1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := p.db.Query(ctx, query, userID)
	if err != nil {
		return nil, p.logger.handleError(err)
	}
	defer rows.Close()

	var permissions data.Permissions

	for rows.Next() {
		var permission string

		err = rows.Scan(&permission)
		if err != nil {
			return nil, p.logger.handleError(err)
		}

		permissions = append(permissions, permission)
	}
	if err = rows.Err(); err != nil {
		return nil, p.logger.handleError(err)
	}

	return permissions, nil
}

// AddForUser Add the provided permission codes for a specific user. Notice that we're using a
// variadic parameter for the codes so that we can assign multiple permissions in a
// single call.
func (p PermissionRepository) AddForUser(userID int64, codes ...string) error {
	query := `
        INSERT INTO users_permissions
        SELECT $1, permissions.id FROM permissions WHERE permissions.code = ANY($2)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := p.db.Exec(ctx, query, userID, codes)
	if err != nil {
		return p.logger.handleError(err)
	}

	return nil
}
