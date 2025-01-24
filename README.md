# Chapter 8. Advanced CRUD Operations

- How to support partial updates to a resource (so that the client only needs to send the data that they want to change).
- How to use optimistic concurrency control to avoid race conditions when two clients try to update the same resource at the same time.
- How to use context timeouts to terminate long-running database queries and prevent unnecessary resource use.

## Handling Partial Updates
Changing the behavior of updateMovieHandler so that it supports partial updates.

It would be nice if we could send a JSON request containing only the change that needs to be applied, instead of all the movie data, like so:
```shell
{"year": 1985}
```

Let’s quickly look at what happens if we try to send this request right now:
```shell
$ curl -X PUT -d '{"year": 1985}' localhost:4000/v1/movies/4
{
    "error": {
        "genres": "must be provided",
        "runtime": "must be provided",
        "title": "must be provided"
    }
}
```

When decoding the request body any fields in our input struct which don’t have a corresponding JSON key/value pair will retain their zero-value.

In the context of partial update this causes a problem. How do we tell the difference between:

- A client providing a key/value pair which has a zero-value value — like {"title": ""} — in which case we want to return a validation error.
- A client not providing a key/value pair in their JSON at all — in which case we want to ‘skip’ updating the field but not send a validation error.

To help answer this, let’s quickly remind ourselves of what the zero-values are for different Go types.


| Go type                          | Zero-value |
|----------------------------------|------------|
| int\*, uint\*, float\*, complex  | 0          |
| string                           | ""         |
| bool                             | false      |
| func, array, slice, map, chan and pointers | nil |


The key thing to notice here is that pointers have the zero-value nil.

So — in theory — we could change the fields in our input struct to be pointers. Then to see if a client has provided a particular key/value pair in the JSON, we can simply check whether the corresponding field in the input struct equals nil or not.

```go
// Use pointers for the Title, Year and Runtime fields.
var input struct {
Title   *string       `json:"title"`   // This will be nil if there is no corresponding key in the JSON.
Year    *int32        `json:"year"`    // Likewise...
Runtime *data.Runtime `json:"runtime"` // Likewise...
Genres  []string      `json:"genres"`  // We don't need to change this because slices already have the zero-value nil.
}
```

### Performing the partial update
```go
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    // Retrieve the movie record as normal.
    movie, err := app.models.Movies.Get(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.notFoundResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // Use pointers for the Title, Year and Runtime fields.
    var input struct {
        Title   *string       `json:"title"`
        Year    *int32        `json:"year"`
        Runtime *data.Runtime `json:"runtime"`
        Genres  []string      `json:"genres"`
    }

    // Decode the JSON as normal.
    err = app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    // If the input.Title value is nil then we know that no corresponding "title" key/
    // value pair was provided in the JSON request body. So we move on and leave the 
    // movie record unchanged. Otherwise, we update the movie record with the new title
    // value. Importantly, because input.Title is a now a pointer to a string, we need 
    // to dereference the pointer using the * operator to get the underlying value
    // before assigning it to our movie record.
    if input.Title != nil {
        movie.Title = *input.Title
    }

    // We also do the same for the other fields in the input struct.
    if input.Year != nil {
        movie.Year = *input.Year
    }
    if input.Runtime != nil {
        movie.Runtime = *input.Runtime
    }
    if input.Genres != nil {
        movie.Genres = input.Genres // Note that we don't need to dereference a slice.
    }

    v := validator.New()
    
    if data.ValidateMovie(v, movie); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    err = app.models.Movies.Update(movie)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

## Optimistic Concurrency Control
Small problem in our updateMovieHandler — there is a race condition if two clients try to update the same movie record at exactly the same time.

To illustrate this, let’s pretend that we have two clients using our API: Alice and Bob. Alice wants to correct the runtime value for The Breakfast Club to 97 mins, and Bob wants to add the genre ‘comedy’ to the same movie.

Now imagine that Alice and Bob send these two update requests at exactly the same time. As we explained in Let’s Go, Go’s http.Server handles each HTTP request in its own goroutine, so when this happens the code in our updateMovieHandler will be running concurrently in two different goroutines.

Let’s step through what could potentially happen in this scenario:

1. Alice’s goroutine calls app.models.Movies.Get() to retrieve a copy of the movie record (which has version number N).
2. Bob’s goroutine calls app.models.Movies.Get() to retrieve a copy of the movie record (which still has version N).
3. Alice’s goroutine changes the runtime to 97 minutes in its copy of the movie record.
4. Bob’s goroutine updates the genres to include ‘comedy’ in its copy of the movie record.
5. Alice’s goroutine calls app.models.Movies.Update() with its copy of the movie record. The movie record is written to the database and the version number is incremented to N+1.
6. Bob’s goroutine calls app.models.Movies.Update() with its copy of the movie record. The movie record is written to the database and the version number is incremented to N+2.

Despite making two separate updates, only Bob’s update will be reflected in the database at the end because the two goroutines were racing each other to make the change. Alice’s update to the movie runtime will be lost when Bob’s update overwrites it with the old runtime value. And this happens silently — there’s nothing to inform either Alice or Bob of the problem.

> This specific type of race condition is known as a data race.

### Preventing the data race
There are a couple of options, but the simplest and cleanest approach in this case is to use a form of optimistic locking based on the version number in our movie record.

The fix works like this:

- Alice and Bob’s goroutines both call app.models.Movies.Get() to retrieve a copy of the movie record. Both of these records have the version number N.
- Alice and Bob’s goroutines make their respective changes to the movie record.
- Alice and Bob’s goroutines call app.models.Movies.Update() with their copies of the movie record. But the update is only executed if the version number in the database is still N. If it has changed, then we don’t execute the update and send the client an error message instead.

This means that the first update request that reaches our database will succeed, and whoever is making the second update will receive an error message instead of having their change applied.

To make this work, we’ll need to change the SQL statement for updating a movie so that it looks like this:
```sql
UPDATE movies
SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
WHERE id = $5 AND version = $6
RETURNING version
```

Notice that in the WHERE clause we’re now looking for a record with a specific ID and a specific version number?
If no matching record can be found, this query will result in a sql.ErrNoRows error and we know that the version number has been changed (or the record has been deleted completely).

### Implementing optimistic locking
```go
func (m MovieModel) Update(movie *Movie) error {
    // Add the 'AND version = $6' clause to the SQL query.
    query := `
        UPDATE movies 
        SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
        WHERE id = $5 AND version = $6
        RETURNING version`

    args := []any{
        movie.Title,
        movie.Year,
        movie.Runtime,
        pq.Array(movie.Genres),
        movie.ID,
        movie.Version, // Add the expected movie version.
    }

    // Execute the SQL query. If no matching row could be found, we know the movie 
    // version has changed (or the record has been deleted) and we return our custom
    // ErrEditConflict error.
    err := m.DB.QueryRow(query, args...).Scan(&movie.Version)
    if err != nil {
        switch {
        case errors.Is(err, sql.ErrNoRows):
            return ErrEditConflict
        default:
            return err
        }
    }

    return nil
}
```

#### Round-trip locking
One of the nice things about the optimistic locking pattern that we’ve used here is that you can extend it so the client passes the version number that they expect in an If-Not-Match or X-Expected-Version header.

In certain applications, this can be useful to help the client ensure they are not sending their update request based on outdated information.

```go
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    movie, err := app.models.Movies.Get(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.notFoundResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // If the request contains a X-Expected-Version header, verify that the movie 
    // version in the database matches the expected version specified in the header.
    if r.Header.Get("X-Expected-Version") != "" {
        if strconv.Itoa(int(movie.Version)) != r.Header.Get("X-Expected-Version") {
            app.editConflictResponse(w, r)
            return
        }
    }

    ...
}
```

#### Locking on other fields or types
sing an incrementing integer version number as the basis for an optimistic lock is safe and computationally cheap. I’d recommend using this approach unless you have a specific reason not to.

As an alternative, you could use a last_updated timestamp as the basis for the lock. But this is less safe — there’s the theoretical possibility that two clients could update a record at exactly the same time, and using a timestamp also introduces the risk of further problems if your server’s clock is wrong or becomes wrong over time.

If it’s important to you that the version identifier isn’t guessable, then a good option is to use a high-entropy random string such as a UUID in the version field. PostgreSQL has a UUID type and the uuid-ossp extension.

```sql
UPDATE movies 
SET title = $1, year = $2, runtime = $3, genres = $4, version = uuid_generate_v4()
WHERE id = $5 AND version = $6
RETURNING version
```

## Managing SQL Query Timeouts
Go provides context-aware variants of Exec() and QueryRow() methods: ExecContext() and QueryRowContext(). These accept a context.Context instance to terminate long-running database queries, freeing up resources and logging errors.


### Mimicking a long-running query
Let’s start by adapting our database model’s Get() method so that it mimics a long-running query. Specifically, we’ll update our SQL query to return a pg_sleep(8) value, which will make PostgreSQL sleep for 8 seconds before returning its result.

```go
func (m MovieModel) Get(id int64) (*Movie, error) {
    if id < 1 {
        return nil, ErrRecordNotFound
    }

    // Update the query to return pg_sleep(8) as the first value.
    query := `
        SELECT pg_sleep(8), id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE id = $1`

    var movie Movie

    // Importantly, update the Scan() parameters so that the pg_sleep(8) return value 
    // is scanned into a []byte slice.
    err := m.DB.QueryRow(query, id).Scan(
        &[]byte{}, // Add this line.
        &movie.ID,
        &movie.CreatedAt,
        &movie.Title,
        &movie.Year,
        &movie.Runtime,
        pq.Array(&movie.Genres),
        &movie.Version,
    )

    if err != nil {
        switch {
        case errors.Is(err, sql.ErrNoRows):
            return nil, ErrRecordNotFound
        default:
            return nil, err
        }
    }

    return &movie, nil
}
```

If you restart the application and make a request to the GET /v1/movies/:id endpoint, you should find that the request hangs for 8 seconds before you finally get a successful response

### Adding a query timeout

Now that we’ve got some code that mimics a long-running query, let’s enforce a timeout so that the SQL query is automatically canceled if it doesn’t complete within 3 seconds.

To do this we need to:
- Use the context.WithTimeout() function to create a context.Context instance with a 3-second timeout deadline.
- Execute the SQL query using the QueryRowContext() method, passing the context.Context instance as a parameter.

```go
func (m MovieModel) Get(id int64) (*Movie, error) {
    if id < 1 {
        return nil, ErrRecordNotFound
    }

    query := `
        SELECT pg_sleep(8), id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE id = $1`

    var movie Movie

    // Use the context.WithTimeout() function to create a context.Context which carries a
    // 3-second timeout deadline. Note that we're using the empty context.Background() 
    // as the 'parent' context.
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

    // Importantly, use defer to make sure that we cancel the context before the Get()
    // method returns.
    defer cancel()

    // Use the QueryRowContext() method to execute the query, passing in the context 
    // with the deadline as the first argument. 
    err := m.DB.QueryRowContext(ctx, query, id).Scan(
        &[]byte{},
        &movie.ID,
        &movie.CreatedAt,
        &movie.Title,
        &movie.Year,
        &movie.Runtime,
        pq.Array(&movie.Genres),
        &movie.Version,
    )

    if err != nil {
        switch {
        case errors.Is(err, sql.ErrNoRows):
            return nil, ErrRecordNotFound
        default:
            return nil, err
        }
    }

    return &movie, nil
}
```

- The defer cancel() line is necessary because it ensures that the resources associated with our context will always be released before the Get() method returns, thereby preventing a memory leak. Without it, the resources won’t be released until either the 3-second timeout is hit or the parent context (which in this specific example is context.Background()) is canceled.
- The timeout countdown begins from the moment that the context is created using context.WithTimeout(). Any time spent executing code between creating the context and calling QueryRowContext() will count towards the timeout.

```shell
$ curl -w '\nTime: %{time_total}s \n' localhost:4000/v1/movies/1
{
    "error": "the server encountered a problem and could not process your request"
}

Time: 3.025179s
```

### Timeouts outside of PostgreSQL
There’s another important thing to point out here: it’s possible that the timeout deadline will be hit before the PostgreSQL query even starts.

Earlier in the book we configured our sql.DB connection pool to allow a maximum of 25 open connections.

If all those connections are in-use, then any additional queries will be ‘queued’ by sql.DB until a connection becomes available. In this scenario — or any other which causes a delay — it’s possible that the timeout deadline will be hit before a free database connection even becomes available.

Demonstrate this in our application by setting the maximum open connections to 1 and making two concurrent requests
```shell
$ curl localhost:4000/v1/movies/1 & curl localhost:4000/v1/movies/1 &
[1] 33221
[2] 33222

$ {
    "error": "the server encountered a problem and could not process your request"
}

{
    "error": "the server encountered a problem and could not process your request"
}
```






