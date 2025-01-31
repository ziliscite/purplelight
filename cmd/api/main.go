package main

import (
	"context"
	"expvar"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ziliscite/purplelight/internal/mailer"
	"github.com/ziliscite/purplelight/internal/repository"
	"github.com/ziliscite/purplelight/internal/vcs"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"
)

// Remember, our version number is just a constant string (for now).
var (
	version = vcs.Version()
)

// Add a models field to hold our new Models struct.
// Include a sync.WaitGroup in the application struct. The zero-value for a
// sync.WaitGroup type is a valid, useable, sync.WaitGroup with a 'counter' value of 0,
// so we don't need to do anything else to initialize it before we can use it.
type application struct {
	config Config
	logger *slog.Logger
	mailer mailer.Mailer
	repos  repository.Repositories
	wg     sync.WaitGroup
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

	// Make expvar to hold our metrics data.
	initializeMetrics(db)

	// Defer a call to db.Close() so that the connection pool is closed before the
	// main() function exits.
	defer db.Close()

	// Use the data.NewModels() function to initialize a Models struct, passing in the
	// connection pool as a parameter.
	app := &application{
		config: cfg,
		logger: logger,
		repos:  repository.NewRepositories(db, logger),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	// Call app.serve() to start the server.
	err = app.serve()
	if err != nil {
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

func initializeMetrics(db *pgxpool.Pool) {
	// Publish a new "version" variable in the expvar handler containing our application
	// version number (currently the constant "1.0.0").
	expvar.NewString("version").Set(version)

	// Publish the number of active goroutines.
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	// Publish the database connection pool statistics.
	expvar.Publish("database", expvar.Func(func() any {
		s := db.Stat()
		var stats struct {
			AcquireCount            int64 `json:"acquire_count"`
			AcquiredConns           int32 `json:"acquired_conns"`
			CanceledAcquireCount    int64 `json:"canceled_acquire_count"`
			ConstructingConns       int32 `json:"constructing_conns"`
			EmptyAcquireCount       int64 `json:"empty_acquire_count"`
			IdleConns               int32 `json:"idle_conns"`
			MaxConns                int32 `json:"max_conns"`
			TotalConns              int32 `json:"total_conns"`
			NewConnsCount           int64 `json:"new_conns_count"`
			MaxLifetimeDestroyCount int64 `json:"max_lifetime_destroy_count"`
			MaxIdleDestroyCount     int64 `json:"max_idle_destroy_count"`
		}

		// Map pgxpool.Stat method calls to the struct fields
		stats.AcquireCount = s.AcquireCount()
		stats.AcquiredConns = s.AcquiredConns()
		stats.CanceledAcquireCount = s.CanceledAcquireCount()
		stats.ConstructingConns = s.ConstructingConns()
		stats.EmptyAcquireCount = s.EmptyAcquireCount()
		stats.IdleConns = s.IdleConns()
		stats.MaxConns = s.MaxConns()
		stats.TotalConns = s.TotalConns()
		stats.NewConnsCount = s.NewConnsCount()
		stats.MaxLifetimeDestroyCount = s.MaxLifetimeDestroyCount()
		stats.MaxIdleDestroyCount = s.MaxIdleDestroyCount()

		return stats
	}))

	// Publish the current Unix timestamp.
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))
}
