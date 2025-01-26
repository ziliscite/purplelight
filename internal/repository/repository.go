package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

// Repositories Create a Models struct which wraps the MovieModel. We'll add other models to this,
// like a UserModel and PermissionModel, as our build progresses.
type Repositories struct {
	Anime AnimeRepository
	User  UserRepository
}

// NewRepositories For ease of use, we also add a New() method which returns a Models struct containing
// the initialized MovieModel.
func NewRepositories(db *pgxpool.Pool, logger *slog.Logger) Repositories {
	dblogger := &dbLogger{logger}
	return Repositories{
		Anime: NewAnimeRepository(db, dblogger),
		User:  NewUserRepository(db, dblogger),
	}
}
