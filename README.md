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








