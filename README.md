# Chapter 11. Graceful Shutdown

In this next section of the book we’re going to talk about an important but often overlooked topic: `how to safely stop your running application`.

At the moment, when we stop our API application (usually by pressing Ctrl+C) it is terminated immediately with no opportunity for in-flight HTTP requests to complete. This isn’t ideal for two reasons:

- It means that clients won’t receive responses to their in-flight requests — all they will experience is a hard closure of the HTTP connection.
- Any work being carried out by our handlers may be left in an incomplete state.

We’re going to mitigate these problems by adding graceful shutdown functionality to our application, so that in-flight HTTP requests have the opportunity to finish being processed `before` the application is terminated.

- Shutdown signals — what they are, how to send them, and how to listen for them in your API application.
- How to use these signals to trigger a graceful shutdown of the HTTP server using Go’s Shutdown() method.

## Sending Shutdown Signals
When our application is running, we can terminate it at any time by sending it a specific signal. A common way to do this, which you’ve probably been using, is by pressing Ctrl+C on your keyboard to send an interrupt signal — also known as a SIGINT.

There are more signal:

| Signal | Description | Keyboard shortcut | Catchable |
| --- | --- | --- | --- |
| SIGINT | Interrupt from keyboard | Ctrl+C | Yes |
| SIGQUIT | Quit from keyboard | Ctrl+\ | Yes |
| SIGKILL | Kill process (terminate immediately) | - | No |
| SIGTERM | Terminate process in orderly manner | - | Yes |

Catachable signals can be intercepted by our application and either ignored, or used to trigger a certain action (such as a graceful shutdown).

Try running our app normally:
```shell
$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
```

Doing this should start a process with the name api on your machine. You can use the pgrep command to verify that this process exists, like so:
```shell
$ pgrep -l api
4414 api
```

Once that’s confirmed, go ahead and try sending a SIGKILL signal to the api process using the pkill command like so:
```shell
$ pkill -SIGKILL api
```

If you go back to the terminal window that is running the API application, you should see that it has been terminated and the final line in the output stream is signal: killed. Similar to:
```shell
$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
signal: killed
```

Sending a SIGTERM signal instead:
```shell
$ pkill -SIGTERM api
```

```shell
$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
signal: terminated
```

Try sending a SIGQUIT signal — either by pressing Ctrl+\ on your keyboard or running pkill -SIGQUIT api. This will cause the application to exit with a stack dump
```shell
$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
SIGQUIT: quit
PC=0x46ebe1 m=0 sigcode=0

goroutine 0 [idle]:
runtime.futex(0x964870, 0x80, 0x0, 0x0, 0x0, 0x964720, 0x7ffd551034f8, 0x964420, 0x7ffd55103508, 0x40dcbf, ...)
        /usr/local/go/src/runtime/sys_linux_amd64.s:579 +0x21
runtime.futexsleep(0x964870, 0x0, 0xffffffffffffffff)
        /usr/local/go/src/runtime/os_linux.go:44 +0x46
runtime.notesleep(0x964870)
        /usr/local/go/src/runtime/lock_futex.go:159 +0x9f
runtime.mPark()
        /usr/local/go/src/runtime/proc.go:1340 +0x39
runtime.stopm()
        /usr/local/go/src/runtime/proc.go:2257 +0x92
runtime.findrunnable(0xc00002c000, 0x0)
        /usr/local/go/src/runtime/proc.go:2916 +0x72e
runtime.schedule()
        /usr/local/go/src/runtime/proc.go:3125 +0x2d7
runtime.park_m(0xc000000180)
        /usr/local/go/src/runtime/proc.go:3274 +0x9d
runtime.mcall(0x0)
        /usr/local/go/src/runtime/asm_amd64.s:327 +0x5b
...
```

We can see that these signals are effective in terminating our application — but the problem we have is that they all cause our application to exit immediately.

Fortunately, Go provides tools in the os/signals package that we can use to intercept catchable signals and trigger a graceful shutdown of our application.

## Intercepting Shutdown Signals
Before we get into the nuts and bolts of how to intercept signals, let’s move the code related to our http.Server out of the main() function and into a separate file.

create a new cmd/api/server.go
```go
func (app *application) serve() error {
    // Declare a HTTP server using the same settings as in our main() function.
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%d", app.config.port),
        Handler:      app.routes(),
        IdleTimeout:  time.Minute,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
    }

    // Likewise log a "starting server" message.
    app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

    // Start the server as normal, returning any error.
    return srv.ListenAndServe()
}
```

```go
// Call app.serve() to start the server.
err = app.serve()
if err != nil {
    logger.Error(err.Error())
    os.Exit(1)
}
```

### Catching SIGINT and SIGTERM signals
The next thing that we want to do is update our application so that it ‘catches’ any SIGINT and SIGTERM signals.

To catch the signals, we’ll need to spin up a background goroutine which runs for the lifetime of our application. In this background goroutine, we can use the signal.Notify() function to listen for specific signals and relay them to a channel for further processing.

```go
func (app *application) serve() error {
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%d", app.config.port),
        Handler:      app.routes(),
        IdleTimeout:  time.Minute,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
    }

    // Start a background goroutine.
    go func() {
        // Create a quit channel which carries os.Signal values.
        quit := make(chan os.Signal, 1)

        // Use signal.Notify() to listen for incoming SIGINT and SIGTERM signals and 
        // relay them to the quit channel. Any other signals will not be caught by
        // signal.Notify() and will retain their default behavior.
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

        // Read the signal from the quit channel. This code will block until a signal is
        // received.
        s := <-quit

        // Log a message to say that the signal has been caught. Notice that we also
        // call the String() method on the signal to get the signal name and include it
        // in the log entry attributes.
        app.logger.Info("caught signal", "signal", s.String())

        // Exit the application with a 0 (success) status code.
        os.Exit(0)
    }()

    // Start the server as normal.
    app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

    return srv.ListenAndServe()
}
```

> Our quit channel is a buffered channel with size 1.

We need to use a buffered channel here because signal.Notify() does not wait for a receiver to be available when sending a signal to the quit channel. If we had used a regular (non-buffered) channel here instead, a signal could be ‘missed’ if our quit channel is not ready to receive at the exact moment that the signal is sent. By using a buffered channel, we avoid this problem and ensure that we never miss a signal.

Run the application and then press Ctrl+C on your keyboard to send a SIGINT signal. You should see a "caught signal" 
```go
$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
time=2023-09-10T10:59:14.345+02:00 level=INFO msg="caught signal" signal=interrupt
```

You can also restart the application and try sending a SIGTERM signal.
```go
$ pkill -SIGTERM api

$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
time=2023-09-10T10:59:14.345+02:00 level=INFO msg="caught signal" signal=terminated
```

In contrast, sending a SIGKILL or SIGQUIT signal will continue to cause the application to exit immediately without the signal being caught.

## Executing the Shutdown
We’re going to update our application so that the SIGINT and SIGTERM signals we intercept trigger a graceful shutdown of our API.

Specifically, after receiving one of these signals we will call the Shutdown() method on our HTTP server.

> Shutdown gracefully shuts down the server without interrupting any active connections. Shutdown works by first closing all open listeners, then closing all idle connections, and then waiting indefinitely for connections to return to idle and then shut down.

```go
func (app *application) serve() error {
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%d", app.config.port),
        Handler:      app.routes(),
        IdleTimeout:  time.Minute,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
    }

    // Create a shutdownError channel. We will use this to receive any errors returned
    // by the graceful Shutdown() function.
    shutdownError := make(chan error)

    go func() {
        // Intercept the signals, as before.
        quit := make(chan os.Signal, 1)
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
        s := <-quit

        // Update the log entry to say "shutting down server" instead of "caught signal".
        app.logger.Info("shutting down server", "signal", s.String())

        // Create a context with a 30-second timeout.
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        // Call Shutdown() on our server, passing in the context we just made.
        // Shutdown() will return nil if the graceful shutdown was successful, or an
        // error (which may happen because of a problem closing the listeners, or 
        // because the shutdown didn't complete before the 30-second context deadline is
        // hit). We relay this return value to the shutdownError channel.
        shutdownError <- srv.Shutdown(ctx)
    }()

    app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

    // Calling Shutdown() on our server will cause ListenAndServe() to immediately 
    // return a http.ErrServerClosed error. So if we see this error, it is actually a
    // good thing and an indication that the graceful shutdown has started. So we check 
    // specifically for this, only returning the error if it is NOT http.ErrServerClosed. 
    err := srv.ListenAndServe()
    if !errors.Is(err, http.ErrServerClosed) {
        return err
    }

    // Otherwise, we wait to receive the return value from Shutdown() on the  
    // shutdownError channel. If return value is an error, we know that there was a
    // problem with the graceful shutdown and we return the error.
    err = <-shutdownError
    if err != nil {
        return err
    }

    // At this point we know that the graceful shutdown completed successfully and we 
    // log a "stopped server" message.
    app.logger.Info("stopped server", "addr", srv.Addr)

    return nil
}
```

At first glance this code might seem a bit complex, but at a high-level what it’s doing can be summarized very simply: when we receive a SIGINT or SIGTERM signal, we instruct our server to stop accepting any new HTTP requests, and give any in-flight requests a ‘grace period’ of 30 seconds to complete before the application is terminated.

It’s important to be aware that the Shutdown() method does not wait for any background tasks to complete, nor does it close hijacked long-lived connections like WebSockets. Instead, you will need to implement your own logic to coordinate a graceful shutdown of these things.

To help demonstrate the graceful shutdown functionality, you can add a 4 second sleep delay to the healthcheckHandler method
```go
func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
    env := envelope{
        "status": "available",
        "system_info": map[string]string{
            "environment": app.config.env,
            "version":     version,
        },
    }

    // Add a 4 second delay.
    time.Sleep(4 * time.Second)

    err := app.writeJSON(w, http.StatusOK, env, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

Then start the API, and in another terminal window issue a request to the healthcheck endpoint followed by a SIGTERM signal.
```shell
$ curl localhost:4000/v1/healthcheck & pkill -SIGTERM api
```

When shutting down, after a 4 second delay
```shell
$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
time=2023-09-10T10:59:14.722+02:00 level=INFO msg="shutting down server" signal=terminated
time=2023-09-10T10:59:18.722+02:00 level=INFO msg="stopped server" addr=:4000
```



