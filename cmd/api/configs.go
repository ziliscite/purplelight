package main

import (
	"flag"
	"github.com/joho/godotenv"
	"log"
	"os"
	"sync"
	"time"
)

// Config Defines an config struct to hold all the configuration settings for our application.
// For now, the only configuration settings will be the network port that we want the
// server to listen on, and the name of the current operating env for the
// application (development, staging, production, etc.). We will readBody in these
// configuration settings from command-line flags when the application starts.
type Config struct {
	port int
	env  string
	db   struct {
		dsn string
		// Add maxOpenConns, maxIdleConns and maxIdleTime fields to hold the configuration
		// settings for the connection pool.
		maxConns    int
		maxIdleTime time.Duration
	}
	// Add a new limiter struct containing fields for the requests-per-second and burst
	// values, and a boolean field which we can use to enable/disable rate limiting
	// altogether.
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
}

var (
	instance Config
	once     sync.Once
)

// GetConfig returns the singleton instance of Config
func GetConfig() Config {
	once.Do(func() {
		instance = Config{}

		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}

		// Read the value of the port and env command-line flags into the config struct. We
		// default to using the port number 4000 and the environment "development" if no
		// corresponding flags are provided.
		flag.IntVar(&instance.port, "port", 4000, "API server port")
		flag.StringVar(&instance.env, "env", "development", "Environment (development|staging|production)")

		// Read the DSN value from the db-dsn command-line flag into the config struct. We
		// default to using our development DSN if no flag is provided.
		flag.StringVar(&instance.db.dsn, "db-dsn", os.Getenv("PURPLELIGHT_DB_DSN"), "PostgreSQL DSN")

		// Read the connection pool settings from command-line flags into the config struct.
		// Notice that the default values we're using are the ones we discussed above?
		flag.IntVar(&instance.db.maxConns, "db-max-open-conns", 25, "PostgreSQL max connections")
		flag.DurationVar(&instance.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

		// Create command line flags to read the setting values into the config struct.
		// Notice that we use true as the default for the 'enabled' setting?
		flag.Float64Var(&instance.limiter.rps, "limiter-rps", 5, "Rate limiter maximum requests per second")
		flag.IntVar(&instance.limiter.burst, "limiter-burst", 10, "Rate limiter maximum burst")
		flag.BoolVar(&instance.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

		flag.Parse()
	})

	return instance
}

// Port Returns the port number that the server should listen to on.
func (c *Config) Port() int {
	return c.port
}

// Env Returns the name of the current operating env for the application.
func (c *Config) Env() string {
	return c.env
}

func (c *Config) DSN() string {
	return c.db.dsn
}
