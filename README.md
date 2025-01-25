# Chapter 10. Rate Limiting
If you’re building an API for public use, then it’s quite likely that you’ll want to implement some form of rate limiting to prevent clients from making too many requests too quickly, and putting excessive strain on your server.

In this section of the book we’re going to create some middleware to help with that.

Essentially, we want this middleware to check how many requests have been received in the last ‘N’ seconds and — if there have been too many — then it should send the client a 429 Too Many Requests response. We’ll position this middleware before our main application handlers, so that it carries out this check before we do any expensive processing like decoding a JSON request body or querying our database.

- About the principles behind token-bucket rate-limiter algorithms and how we can apply them in the context of an API or web application.
- How to create middleware to rate-limit requests to your API endpoints, first by making a single rate global limiter, then extending it to support per-client limiting based on IP address.
- How to make rate limiter behavior configurable at runtime, including disabling the rate limiter altogether for testing purposes.

## Global Rate Limiting
This will consider all the requests that our API receives.

We will use `x/time/rate` package, which provides a tried-and-tested implementation of a token bucket rate limiter.
```shell
$ go get golang.org/x/time/rate@latest
go: downloading golang.org/x/time v0.6.0
go: added golang.org/x/time v0.6.0
```

> A Limiter controls how frequently events are allowed to happen. It implements a “token bucket” of size b, initially full and refilled at rate r tokens per second.

Putting that into the context of our API application...

- We will have a bucket that starts with b tokens in it.
- Each time we receive a HTTP request, we will remove one token from the bucket.
- Every 1/r seconds, a token is added back to the bucket — up to a maximum of b total tokens.
- If we receive a HTTP request and the bucket is empty, then we should return a 429 Too Many Requests response.

In order to create a token bucket rate limiter from x/time/rate, we will need to use the NewLimiter() function. This has a signature which looks like this:
```go
// Note that the Limit type is an 'alias' for float64.
func NewLimiter(r Limit, b int) *Limiter
```

So if we want to create a rate limiter which allows an average of 2 requests per second, with a maximum of 4 requests in a single ‘burst’, we could do so with the following code:
```go
// Allow 2 requests per second, with a maximum of 4 requests in a burst.
limiter := rate.NewLimiter(2, 4)
```

### Enforcing a global rate limit
One of the nice things about the middleware pattern that we are using is that it is straightforward to include ‘initialization’ code which only runs once when we wrap something with the middleware, rather than running on every request that the middleware handles.
```go
func (app *application) exampleMiddleware(next http.Handler) http.Handler {
    
    // Any code here will run only once, when we wrap something with the middleware. 

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        
        // Any code here will run for every request that the middleware handles.

        next.ServeHTTP(w, r)
    })
}
```

We’ll make a new rateLimit() middleware method which creates a new rate limiter as part of the ‘initialization’ code, and then uses this rate limiter for every request that it subsequently handles.
```go
func (app *application) rateLimit(next http.Handler) http.Handler {
    // Initialize a new rate limiter which allows an average of 2 requests per second, 
    // with a maximum of 4 requests in a single ‘burst’.
    limiter := rate.NewLimiter(2, 4)

    // The function we are returning is a closure, which 'closes over' the limiter 
    // variable.
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Call limiter.Allow() to see if the request is permitted, and if it's not, 
        // then we call the rateLimitExceededResponse() helper to return a 429 Too Many
        // Requests response (we will create this helper in a minute).
        if !limiter.Allow() {
            app.rateLimitExceededResponse(w, r)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

In this code, whenever we call the Allow() method on the rate limiter exactly one token will be consumed from the bucket. If there are no tokens left in the bucket, then Allow() will return false and that acts as the trigger for us send the client a 429 Too Many Requests response.

```go
func (app *application) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
    message := "rate limit exceeded"
    app.errorResponse(w, r, http.StatusTooManyRequests, message)
}
```

in the cmd/api/routes.go file we want to add the rateLimit() middleware to our middleware chain. This should come `after` our panic recovery middleware (so that any panics in rateLimit() are recovered), but otherwise we want it to be used as early as possible to prevent unnecessary work for our server.
```go
func (app *application) routes() http.Handler {
    router := httprouter.New()

    router.NotFound = http.HandlerFunc(app.notFoundResponse)
    router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

    router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

    router.HandlerFunc(http.MethodGet, "/v1/movies", app.listMoviesHandler)
    router.HandlerFunc(http.MethodPost, "/v1/movies", app.createMovieHandler)
    router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler)
    router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.updateMovieHandler)
    router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.deleteMovieHandler)

    // Wrap the router with the rateLimit() middleware.
    return app.recoverPanic(app.rateLimit(router))
}
```

Restart the API, then in another terminal window execute the following command to issue a batch of 6 requests to our GET /v1/healthcheck endpoint in quick succession.
```shell
$ for i in {1..6}; do curl http://localhost:4000/v1/healthcheck; done
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "error": "rate limit exceeded"
}
{
    "error": "rate limit exceeded"
}
```
We can see from this that the first 4 requests succeed, due to our limiter being set up to permit a ‘burst’ of 4 requests in quick succession. But once those 4 requests were used up, the tokens in the bucket ran out and our API began to return the "rate limit exceeded" error response instead.

## IP-based Rate Limiting
Generally, its more common to want an individual rate limiter for each client than global rate limiting, so that one bad client making too many requests doesn’t affect all the others.

A conceptually straightforward way to implement this is to create an in-memory map of rate limiters, using the IP address for each client as the map key.

// I assume for distributed systems, you would have a distributed key-value store, like Redis or Memcached. Or using a rate limiter from a reverse proxy like Nginx or HAProxy, which would have a global rate limiter.

Each time a new client makes a request to our API, we will initialize a new rate limiter and add it to the map. For any subsequent requests, we will retrieve the client’s rate limiter from the map and check whether the request is permitted by calling its Allow() method, just like we did before.

Jump into the code and update our `rateLimit()` middleware
```go
func (app *application) rateLimit(next http.Handler) http.Handler {
    // Declare a mutex and a map to hold the clients' IP addresses and rate limiters.
    var (
        mu      sync.Mutex
        clients = make(map[string]*rate.Limiter)
    )

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract the client's IP address from the request.
        ip, _, err := net.SplitHostPort(r.RemoteAddr)
        if err != nil {
            app.serverErrorResponse(w, r, err)
            return
        }

        // Lock the mutex to prevent this code from being executed concurrently.
        mu.Lock()

        // Check to see if the IP address already exists in the map. If it doesn't, then
        // initialize a new rate limiter and add the IP address and limiter to the map.
        if _, found := clients[ip]; !found {
            clients[ip] = rate.NewLimiter(2, 4)
        }

        // Call the Allow() method on the rate limiter for the current IP address. If
        // the request isn't allowed, unlock the mutex and send a 429 Too Many Requests
        // response, just like before.
        if !clients[ip].Allow() {
            mu.Unlock()
            app.rateLimitExceededResponse(w, r)
            return
        }

        // Very importantly, unlock the mutex before calling the next handler in the
        // chain. Notice that we DON'T use defer to unlock the mutex, as that would mean
        // that the mutex isn't unlocked until all the handlers downstream of this 
        // middleware have also returned.
        mu.Unlock()

        next.ServeHTTP(w, r)
    })
}
```

### Deleting old limiters
The code above will work, but there’s a slight problem — the clients map will grow indefinitely, taking up more and more resources with every new IP address and rate limiter that we add.

To prevent this, let’s update our code so that we also record the last seen time for each client. We can then run a background goroutine in which we periodically delete any clients that we haven’t been seen recently from the clients map.
```go
func (app *application) rateLimit(next http.Handler) http.Handler {
    // Define a client struct to hold the rate limiter and last seen time for each
    // client.
    type client struct {
        limiter  *rate.Limiter
        lastSeen time.Time
    }

    var (
        mu sync.Mutex
        // Update the map so the values are pointers to a client struct.
        clients = make(map[string]*client)
    )

    // Launch a background goroutine which removes old entries from the clients map once
    // every minute.
    go func() {
        for {
            time.Sleep(time.Minute)

            // Lock the mutex to prevent any rate limiter checks from happening while
            // the cleanup is taking place.
            mu.Lock()

            // Loop through all clients. If they haven't been seen within the last three
            // minutes, delete the corresponding entry from the map.
            for ip, client := range clients {
                if time.Since(client.lastSeen) > 3*time.Minute {
                    delete(clients, ip)
                }
            }

            // Importantly, unlock the mutex when the cleanup is complete.
            mu.Unlock()
        }
    }()

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip, _, err := net.SplitHostPort(r.RemoteAddr)
        if err != nil {
            app.serverErrorResponse(w, r, err)
            return
        }

        mu.Lock()

        if _, found := clients[ip]; !found {
            // Create and add a new client struct to the map if it doesn't already exist.
            clients[ip] = &client{limiter: rate.NewLimiter(2, 4)}
        }

        // Update the last seen time for the client.
        clients[ip].lastSeen = time.Now()

        if !clients[ip].limiter.Allow() {
            mu.Unlock()
            app.rateLimitExceededResponse(w, r)
            return
        }

        mu.Unlock()

        next.ServeHTTP(w, r)
    })
}
```

Test
```shell
$ for i in {1..6}; do curl  http://localhost:4000/v1/healthcheck; done
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
{
    "error": "rate limit exceeded"
}
{
    "error": "rate limit exceeded"
}
```

### Distributed applications
Using this pattern for rate-limiting will only work if your API application is running on a single-machine. If your infrastructure is distributed, with your application running on multiple servers behind a load balancer, then you’ll need to use an alternative approach.  
If you’re using HAProxy or Nginx as a load balancer or reverse proxy, both of these have built-in functionality for rate limiting that it would probably be sensible to use. Alternatively, you could use a fast database like Redis to maintain a request count for clients, running on a server which all your application servers can communicate with.


## Configuring the Rate Limiters
At the moment our requests-per-second and burst values are hard-coded into the rateLimit() middleware. This is OK, but it would be more flexible if they were configurable at runtime instead.

Likewise, it would be useful to have an easy way to turn off rate limiting altogether (which is useful when you want to run benchmarks or carry out load testing, when all requests might be coming from a small number of IP addresses).

```go
type config struct {
    port int
    env  string
    db   struct {
        dsn          string
        maxOpenConns int
        maxIdleConns int
        maxIdleTime  time.Duration
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
...

func main() {
    var cfg config

    flag.IntVar(&cfg.port, "port", 4000, "API server port")
    flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

    flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")

    flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
    flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
    flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

    // Create command line flags to read the setting values into the config struct. 
    // Notice that we use true as the default for the 'enabled' setting?
    flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
    flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
    flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

    flag.Parse()

    ...
}
```

```go
func (app *application) rateLimit(next http.Handler) http.Handler {
    
    ...
    
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Only carry out the check if rate limiting is enabled.
        if app.config.limiter.enabled {
            ip, _, err := net.SplitHostPort(r.RemoteAddr)
            if err != nil {
                app.serverErrorResponse(w, r, err)
                return
            }

            mu.Lock()

            if _, found := clients[ip]; !found {
                clients[ip] = &client{
                    // Use the requests-per-second and burst values from the config
                    // struct.
                    limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
                }
            }

            clients[ip].lastSeen = time.Now()

            if !clients[ip].limiter.Allow() {
                mu.Unlock()
                app.rateLimitExceededResponse(w, r)
                return
            }

            mu.Unlock()
        }

        next.ServeHTTP(w, r)
    })
}
```

Once that’s done, let’s try this out by running the API with the -limiter-burst flag and the burst value reduced to 2:
```shell
$ go run ./cmd/api/ -limiter-burst=2
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
```

Similarly, you can try disabling the rate limiter altogether with the -limiter-enabled=false flag like so:
```shell
$ go run ./cmd/api/ -limiter-enabled=false
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
```

// this flag thing, its done at runtime? I guess it would make sense to check it on the middleware
// or else, I would do the checking on the router...


