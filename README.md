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


