# Chapter 7. CRUD Operations

Endpoints  

| Method | URL Pattern | Handler | Action |
| --- | --- | --- | --- |
| GET | /v1/healthcheck | healthcheckHandler | Show application information |
| POST | /v1/movies | createMovieHandler | Create a new movie |
| GET | /v1/movies/:id | showMovieHandler | Show the details of a specific movie |
| PUT | /v1/movies/:id | updateMovieHandler | UpdateAnime the details of a specific movie |
| DELETE | /v1/movies/:id | deleteMovieHandler | Delete a specific movie |

// change movie to anime, of course

Key takeaways
- How to create a database model which isolates all the logic for executing SQL queries against your database.
- How to implement the four basic CRUD (create, read, update and delete) operations on a specific resource in the context of an API.

## Setting up the Movie Model
To the internal/data/movies.go file and create a MovieModel struct type and some placeholder methods for performing basic CRUD (create, read, update and delete).
```go
package data

import (
    "database/sql" // New import
    "time"

    "greenlight.alexedwards.net/internal/validator"
)

...

// Define a MovieModel struct type which wraps a sql.DB connection pool.
type MovieModel struct {
    DB *sql.DB
}

// Add a placeholder method for inserting a new record in the movies table.
func (m MovieModel) Insert(movie *Movie) error {
    return nil
}

// Add a placeholder method for fetching a specific record from the movies table.
func (m MovieModel) GetAnime(id int64) (*Movie, error) {
    return nil, nil
}

// Add a placeholder method for updating a specific record in the movies table.
func (m MovieModel) UpdateAnime(movie *Movie) error {
    return nil
}

// Add a placeholder method for deleting a specific record from the movies table.
func (m MovieModel) Delete(id int64) error {
    return nil
}
```

// I'll put it in a separate package, repository  
// well damn, mine gonna differs quite a lot here, so Imma just skim over it  
// just covering the theory or new things

## Creating a New Movie
Insert() method
```sql
INSERT INTO movies (title, year, runtime, genres) 
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, version
```

There are few things about this query which warrant a bit of explanation.

- It uses $N notation to represent placeholder parameters for the data that we want to insert in the movies table. As we explained in Let’s Go, every time that you pass untrusted input data from a client to a SQL database it’s important to use placeholder parameters to help prevent SQL injection attacks, unless you have a very specific reason for not using them.

- We’re only inserting values for title, year, runtime and genres. The remaining columns in the movies table will be filled with system-generated values at the moment of insertion — the id will be an auto-incrementing integer, and the created_at and version values will default to the current time and 1 respectively.

- At the end of the query we have a RETURNING clause. This is a PostgreSQL-specific clause (it’s not part of the SQL standard) that you can use to return values from any record that is being manipulated by an INSERT, UPDATE or DELETE statement. In this query we’re using it to return the system-generated id, created_at and version values.

### Executing the SQL query
Normally, you would use Go’s Exec() method to execute an INSERT statement against a database table. But because our SQL query is returning a single row of data (thanks to the RETURNING clause), we’ll need to use the QueryRow() method here instead.

```go
// The Insert() method accepts a pointer to a movie struct, which should contain the 
// data for the new record.
func (m MovieModel) Insert(movie *Movie) error {
    // Define the SQL query for inserting a new record in the movies table and returning
    // the system-generated data.
    query := `
        INSERT INTO movies (title, year, runtime, genres) 
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, version`

    // Create an args slice containing the values for the placeholder parameters from 
    // the movie struct. Declaring this slice immediately next to our SQL query helps to
    // make it nice and clear *what values are being used where* in the query.
    args := []any{movie.Title, movie.Year, movie.Runtime, pq.Array(movie.Genres)}

	// Storing the inputs in a slice isn’t strictly necessary, 
	// but as mentioned in the code comments above it’s a nice pattern that can help the clarity of your code.
	
	
    // Use the QueryRow() method to execute the SQL query on our connection pool,
    // passing in the args slice as a variadic parameter and scanning the system-
    // generated id, created_at and version values into the movie struct.
    return m.DB.QueryRow(query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}
```

Because the Insert() method signature takes a `*Movie` pointer as the parameter, when we call `Scan()` to read in the system-generated data we’re updating the values at the location the parameter points to. Essentially, our `Insert()` method mutates the Movie struct that we pass to it and adds the system-generated values to it.

In order to store our movie.Genres value (which is a []string slice) in the database, we need to pass it through the pq.Array() adapter function before executing the SQL query.

Behind the scenes, the pq.Array() adapter takes our []string slice and converts it to a pq.StringArray type. In turn, the pq.StringArray type implements the driver.Valuer and sql.Scanner interfaces necessary to translate our native []string slice to and from a value that our PostgreSQL database can understand and store in a text[] array column.

// I dont use array, I used many-to-many relationship to store tags

### Hooking it up to API handler
```go
func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title   string       `json:"title"`
        Year    int32        `json:"year"`
        Runtime data.Runtime `json:"runtime"`
        Genres  []string     `json:"genres"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    // Note that the movie variable contains a *pointer* to a Movie struct.
    movie := &data.Movie{
        Title:   input.Title,
        Year:    input.Year,
        Runtime: input.Runtime,
        Genres:  input.Genres,
    }

    v := validator.New()

    if data.ValidateMovie(v, movie); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Call the Insert() method on our movies model, passing in a pointer to the 
    // validated movie struct. This will create a record in the database and update the 
    // movie struct with the system-generated information.
    err = app.models.Movies.Insert(movie)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    // When sending a HTTP response, we want to include a Location header to let the 
    // client know which URL they can find the newly-created resource at. We make an  
    // empty http.Header map and then use the Set() method to add a new Location header,
    // interpolating the system-generated ID for our new movie in the URL.
    headers := make(http.Header)
    headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

    // Write a JSON response with a 201 Created status code, the movie data in the 
    // response body, and the Location header.
    err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

The response also includes the Location: /v1/movies/1 header, pointing to the URL which will later represent the movie in our system.

#### $N notation
A nice feature of the PostgreSQL placeholder parameter $N notation is that you can use the same parameter value in multiple places in your SQL statement. For example, it’s perfectly acceptable to write code like this:

```go
// This SQL statement uses the $1 parameter twice, and the value `123` will be used in 
// both locations where $1 appears.
stmt := "UPDATE foo SET bar = $1 + $2 WHERE bar = $1"
err := db.Exec(stmt, 123, 456)
if err != nil {
    ...
}
```

#### Executing multiple statements
Occasionally you might find yourself in the position where you want to execute more than one SQL statement in the same database call, like this:
```go
stmt := `
    UPDATE foo SET bar = true; 
    UPDATE foo SET baz = false;`

err := db.Exec(stmt)
if err != nil {
    ...
}
```

Having multiple statements in the same call is supported by the pq driver, so long as the statements do not contain any `placeholder parameters`. If they do contain placeholder parameters, then you’ll receive the following error message at runtime:
```shell
pq: cannot insert multiple commands into a prepared statement
```

To work around this, you will need to either split out the statements into separate database calls, or if that’s not possible, you can create a custom function in PostgreSQL which acts as a wrapper around the multiple SQL statements that you want to run.

## Fetching a Movie
Get() method
```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE id = $1
```

Because our movies table uses the id column as its primary key, this query will only ever return exactly one database row (or none at all).
```go
func (m MovieModel) Get(id int64) (*Movie, error) {
    // The PostgreSQL bigserial type that we're using for the movie ID starts
    // auto-incrementing at 1 by default, so we know that no movies will have ID values
    // less than that. To avoid making an unnecessary database call, we take a shortcut
    // and return an ErrRecordNotFound error straight away.
    if id < 1 {
        return nil, ErrRecordNotFound
    }

    // Define the SQL query for retrieving the movie data.
    query := `
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE id = $1`

    // Declare a Movie struct to hold the data returned by the query.
    var movie Movie

    // Execute the query using the QueryRow() method, passing in the provided id value  
    // as a placeholder parameter, and scan the response data into the fields of the 
    // Movie struct. Importantly, notice that we need to convert the scan target for the 
    // genres column using the pq.Array() adapter function again.
    err := m.DB.QueryRow(query, id).Scan(
        &movie.ID,
        &movie.CreatedAt,
        &movie.Title,
        &movie.Year,
        &movie.Runtime,
        pq.Array(&movie.Genres),
        &movie.Version,
    )

    // Handle any errors. If there was no matching movie found, Scan() will return 
    // a sql.ErrNoRows error. We check for this and return our custom ErrRecordNotFound 
    // error instead. 
    if err != nil {
        switch {
        case errors.Is(err, sql.ErrNoRows):
            return nil, ErrRecordNotFound
        default:
            return nil, err
        }
    }

    // Otherwise, return a pointer to the Movie struct.
    return &movie, nil
}
```

### Updating the API handler
```go
func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    // Call the Get() method to fetch the data for a specific movie. We also need to 
    // use the errors.Is() function to check if it returns a data.ErrRecordNotFound
    // error, in which case we send a 404 Not Found response to the client.
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

    err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

#### Why not use an unsigned integer for the movie ID?
At the start of the Get() method we have the following code which checks if the movie id parameter is less than 1:
```go
func (m MovieModel) Get(id int64) (*Movie, error) {
    if id < 1 {
        return nil, ErrRecordNotFound
    }

    ...
}

This might have led you to wonder: if the movie ID is never negative, why aren’t we using an unsigned uint64 type to store the ID in our Go code, instead of an int64?

There are two reasons for this.

The first reason is because PostgreSQL doesn’t have unsigned integers. It’s generally sensible to align your Go and database integer types to avoid overflows or other compatibility problems, so because PostgreSQL doesn’t have unsigned integers, this means that we should avoid using uint* types in our Go code for any values that we’re reading/writing to PostgreSQL too.

Instead, it’s best to align the integer types based on the following table:


| PostgreSQL type | Go type                          |
| --------------- | -------------------------------- |
| smallint, smallserial | int16 (-32768 to 32767)         |
| integer, serial       | int32 (-2147483648 to 2147483647) |
| bigint, bigserial     | int64 (-9223372036854775808 to 9223372036854775807) |


## Updating a Movie

### Executing the SQL query
```sql
UPDATE movies
SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
WHERE id = $5
RETURNING version
```

```go
func (m MovieModel) Update(movie *Movie) error {
    // Declare the SQL query for updating the record and returning the new version
    // number.
    query := `
        UPDATE movies 
        SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
        WHERE id = $5
        RETURNING version`

    // Create an args slice containing the values for the placeholder parameters.
    args := []any{
        movie.Title,
        movie.Year,
        movie.Runtime,
        pq.Array(movie.Genres),
        movie.ID,
    }

    // Use the QueryRow() method to execute the query, passing in the args slice as a
    // variadic parameter and scanning the new version value into the movie struct.
    return m.DB.QueryRow(query, args...).Scan(&movie.Version)
}
```

### Creating the API handler
Specifically, we’ll need to:

- Extract the movie ID from the URL using the app.readIDParam() helper.
- Fetch the corresponding movie record from the database using the Get() method that we made in the previous chapter.
- Read the JSON request body containing the updated movie data into an input struct.
- Copy the data across from the input struct to the movie record.
- Check that the updated movie record is valid using the data.ValidateMovie() function.
- Call the Update() method to store the updated movie record in our database.
- Write the updated movie data in a JSON response using the app.writeJSON() helper.

```go
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
    // Extract the movie ID from the URL.
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    // Fetch the existing movie record from the database, sending a 404 Not Found 
    // response to the client if we couldn't find a matching record.
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

    // Declare an input struct to hold the expected data from the client.
    var input struct {
        Title   string       `json:"title"`
        Year    int32        `json:"year"`
        Runtime data.Runtime `json:"runtime"`
        Genres  []string     `json:"genres"`
    }

    // Read the JSON request body data into the input struct.
    err = app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    // Copy the values from the request body to the appropriate fields of the movie
    // record.
    movie.Title = input.Title
    movie.Year = input.Year
    movie.Runtime = input.Runtime
    movie.Genres = input.Genres

    // Validate the updated movie record, sending the client a 422 Unprocessable Entity
    // response if any checks fail.
    v := validator.New()

    if data.ValidateMovie(v, movie); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Pass the updated movie record to our new Update() method.
    err = app.models.Movies.Update(movie)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    // Write the updated movie record in a JSON response.
    err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

Update routes
```go
func (app *application) routes() http.Handler {
    router := httprouter.New()

    router.NotFound = http.HandlerFunc(app.notFoundResponse)
    router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

    router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)
    router.HandlerFunc(http.MethodPost, "/v1/movies", app.createMovieHandler)
    router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler)
    // Add the route for the PUT /v1/movies/:id endpoint.
    router.HandlerFunc(http.MethodPut, "/v1/movies/:id", app.updateMovieHandler)

    return app.recoverPanic(router)
}
```

Example
```shell
$ BODY='{"title":"Black Panther","year":2018,"runtime":"134 mins","genres":["sci-fi","action","adventure"]}'
$ curl -X PUT -d "$BODY" localhost:4000/v1/movies/2
{
    "movie": {
        "id": 2,
        "title": "Black Panther",
        "year": 2018,
        "runtime": "134 mins",
        "genres": [
            "sci-fi",
            "action",
            "adventure"
        ],
        "version": 2
    }
}
```

## Deleting a Movie
`DELETE	/v1/movies/:id	deleteMovieHandler	Delete a specific movie`

- If a movie with the id provided in the URL exists in the database, we want to delete the corresponding record and return a success message to the client.
- If the movie id doesn’t exist, we want to return a 404 Not Found response to the client.

SQL query
```sql
DELETE FROM movies
WHERE id = $1
```

In this case the SQL query returns no rows, so it’s appropriate for us to use Go’s Exec().


Adding the new endpoint
```go
func (m MovieModel) Delete(id int64) error {
    // Return an ErrRecordNotFound error if the movie ID is less than 1.
    if id < 1 {
        return ErrRecordNotFound
    }

    // Construct the SQL query to delete the record.
    query := `
        DELETE FROM movies
        WHERE id = $1`

    // Execute the SQL query using the Exec() method, passing in the id variable as
    // the value for the placeholder parameter. The Exec() method returns a sql.Result
    // object.
    result, err := m.DB.Exec(query, id)
    if err != nil {
        return err
    }

    // Call the RowsAffected() method on the sql.Result object to get the number of rows
    // affected by the query.
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return err
    }

    // If no rows were affected, we know that the movies table didn't contain a record
    // with the provided ID at the moment we tried to delete it. In that case we 
    // return an ErrRecordNotFound error.
    if rowsAffected == 0 {
        return ErrRecordNotFound
    }

    return nil
}
```

```go
func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
    // Extract the movie ID from the URL.
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    // Delete the movie from the database, sending a 404 Not Found response to the
    // client if there isn't a matching record.
    err = app.models.Movies.Delete(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.notFoundResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // Return a 200 OK status code along with a success message.
    err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

