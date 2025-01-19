package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ziliscite/purplelight/internal/repository"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const version = "1.0.0"

// Add a models field to hold our new Models struct.
type application struct {
	config Config
	logger *slog.Logger
	repos  repository.Repositories
}

func main() {
	cfg := GetConfig()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Call the openDB() helper function (see below) to create the connection pool,
	// passing in the config struct. If this returns an error, we log it and exit the
	// application immediately.
	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// Also log a message to say that the connection pool has been successfully
	logger.Info("database connection pool established")

	// Defer a call to db.Close() so that the connection pool is closed before the
	// main() function exits.
	defer db.Close()

	// Use the data.NewModels() function to initialize a Models struct, passing in the
	// connection pool as a parameter.
	app := &application{
		config: cfg,
		logger: logger,
		repos:  repository.NewRepositories(db, logger),
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "addr", srv.Addr, "env", cfg.Env())

	if err := srv.ListenAndServe(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

// The openDB() function returns a sql.DB connection pool.
func openDB(cfg Config) (*pgxpool.Pool, error) {
	// Use sql.Open() to create an empty connection pool, using the DSN from the config
	// struct.
	config, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, err
	}

	// Set the maximum number of open (in-use + idle) connections in the pool. Note that
	// passing a value less than or equal to 0 will mean there is no limit.
	// Set the maximum number of idle connections in the pool. Again, passing a value
	// less than or equal to 0 will mean there is no limit.
	config.MaxConns = int32(cfg.db.maxConns)

	// Set the maximum idle timeout for connections in the pool. Passing a duration less
	// than or equal to 0 will mean that connections are not closed due to their idle time.
	config.MaxConnIdleTime = cfg.db.maxIdleTime

	config.MinConns = 2

	// Create a context with a 5-second timeout deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Use PingContext() to establish a new connection to the database, passing in the
	// context we created above as a parameter. If the connection couldn't be
	// established successfully within the 5 second deadline, then this will return an
	// error. If we get this error, or any other, we close the connection pool and
	// return the error.
	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
