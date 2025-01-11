## Chapter 2.2. A Basic HTTP Server

In this chapter, the book explains how to create a basic HTTP server in Go.
My implementations, albeit a little different, is based on the book's example.

We are using `net/http` to create an HTTP server that listens on port 4000. 

This book instructed us to create a handler, `/v1/healthcheck`, 
that prints a text response to the client. Responses are `status`, `environment`, and `version`.

```go
package main

import (
	"fmt"
	"net/http"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "status: available")
    fmt.Fprintf(w, "environment: %s\n", app.config.env)
    fmt.Fprintf(w, "version: %s\n", version)
}
```

But before we do that, the book first tell us to define a config struct that will hold the port and environment values. 
As well as a logger to write log messages. Then, we define an application struct to hold the config struct and the logger.
```go
package main

const version = "1.0.0"

type config struct {
    port int
    env  string
}

type application struct {
    config config
    logger *slog.Logger
}
```

```go
var cfg config

flag.IntVar(&cfg.port, "port", 4000, "API server port")
flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
flag.Parse()
```

Finally, we define a new servemux and add a `/v1/healthcheck` route which dispatches requests to the `healthcheckHandler` method.
```go
app := &application{
    config: cfg,
    logger: logger,
}

mux := http.NewServeMux()
mux.HandleFunc("/v1/healthcheck", app.healthcheckHandler)

srv := &http.Server{
    Addr:         fmt.Sprintf(":%d", cfg.Port()),
    Handler:      mux,
    IdleTimeout:  time.Minute,
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
    ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
}

logger.Info("starting server", "addr", srv.Addr, "env", cfg.Env())
err := srv.ListenAndServe()
if err != nil {
    logger.Error(err.Error())
    os.Exit(1)
}
```

We can run it by executing the following command:
```bash
go run ./cmd/api
```

Then visiting the following URL: http://localhost:4000/v1/healthcheck

We can also run it by specifying the port and env
```bash
go run ./cmd/api -port=3030 -env=production
```
