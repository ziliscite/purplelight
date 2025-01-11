package main

import (
	"flag"
	"sync"
)

// Config Defines an config struct to hold all the configuration settings for our application.
// For now, the only configuration settings will be the network port that we want the
// server to listen on, and the name of the current operating env for the
// application (development, staging, production, etc.). We will read in these
// configuration settings from command-line flags when the application starts.
type Config struct {
	port int
	env  string
}

var (
	instance Config
	once     sync.Once
)

// GetConfig returns the singleton instance of Config
func GetConfig() Config {
	once.Do(func() {
		instance = Config{}

		// Read the value of the port and env command-line flags into the config struct. We
		// default to using the port number 4000 and the environment "development" if no
		// corresponding flags are provided.
		flag.IntVar(&instance.port, "port", 4000, "API server port")
		flag.StringVar(&instance.env, "env", "development", "Environment (development|staging|production)")
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
