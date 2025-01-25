# Chapter 9. Filtering, Sorting, and Pagination

- Return the details of multiple resources in a single JSON response.
- Accept and apply optional filter parameters to narrow down the returned data set.
- Implement full-text search on your database fields using PostgreSQL’s inbuilt functionality.
- Accept and safely apply sort parameters to change the order of results in the data set.
- Develop a pragmatic, reusable, pattern to support pagination on large data sets, and return pagination metadata in your JSON responses.

## Parsing Query String Parameters
we’re going configure the GET /v1/movies to accept query params, like:
```shell
/v1/movies?title=godfather&genres=crime,drama&page=1&page_size=5&sort=-year
```

If a client sends a query string like this, it is essentially saying to our API: “please return the first 5 records where the movie name includes godfather and the genres include crime and drama, sorted by descending release year”.

> parameter sort=title implies an ascending alphabetical sort on movie title, whereas sort=-title implies a descending sort.

In our case, we’ll need to carry out extra post-processing on some of these query string values too. Specifically:

- The genres parameter will potentially contain multiple comma-separated values — like genres=crime,drama. We will want to split these values apart and store them in a []string slice.
- The page and page_size parameters will contain numbers, and we will want to convert these query string values into Go int types.

In addition to that:

- There are some validation checks that we’ll want to apply to the query string values, like making sure that page and page_size are not negative numbers.
- We want our application to set some sensible default values in case parameters like page, page_size and sort aren’t provided by the client.

### Creating helper functions
```go
// The readString() helper returns a string value from the query string, or the provided
// default value if no matching key could be found.
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
    // Extract the value for a given key from the query string. If no key exists this
    // will return the empty string "". 
    s := qs.Get(key)

    // If no key exists (or the value is empty) then return the default value.
    if s == "" {
        return defaultValue
    }

    // Otherwise return the string.
    return s
}

// The readCSV() helper reads a string value from the query string and then splits it 
// into a slice on the comma character. If no matching key could be found, it returns
// the provided default value.
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
    // Extract the value from the query string.
    csv := qs.Get(key)

    // If no key exists (or the value is empty) then return the default value.
    if csv == "" {
        return defaultValue
    }

    // Otherwise parse the value into a []string slice and return it.
    return strings.Split(csv, ",")
}


// The readInt() helper reads a string value from the query string and converts it to an 
// integer before returning. If no matching key could be found it returns the provided 
// default value. If the value couldn't be converted to an integer, then we record an 
// error message in the provided Validator instance. 
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
    // Extract the value from the query string.
    s := qs.Get(key)

    // If no key exists (or the value is empty) then return the default value.
    if s == "" {
        return defaultValue
    }

    // Try to convert the value to an int. If this fails, add an error message to the 
    // validator instance and return the default value.
    i, err := strconv.Atoi(s)
    if err != nil {
        v.AddError(key, "must be an integer value")
        return defaultValue
    }

    // Otherwise, return the converted integer value.
    return i
}
```

### Adding the API handler and route
```go
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
    // To keep things consistent with our other handlers, we'll define an input struct
    // to hold the expected values from the request query string.
    var input struct {
        Title    string
        Genres   []string
        Page     int
        PageSize int
        Sort     string
    }

    // Initialize a new Validator instance.
    v := validator.New()

    // Call r.URL.Query() to get the url.Values map containing the query string data.
    qs := r.URL.Query()

    // Use our helpers to extract the title and genres query string values, falling back
    // to defaults of an empty string and an empty slice respectively if they are not 
    // provided by the client.
    input.Title = app.readString(qs, "title", "")
    input.Genres = app.readCSV(qs, "genres", []string{})

    // Get the page and page_size query string values as integers. Notice that we set 
    // the default page value to 1 and default page_size to 20, and that we pass the 
    // validator instance as the final argument here. 
    input.Page = app.readInt(qs, "page", 1, v)
    input.PageSize = app.readInt(qs, "page_size", 20, v)

    // Extract the sort query string value, falling back to "id" if it is not provided
    // by the client (which will imply a ascending sort on movie ID).
    input.Sort = app.readString(qs, "sort", "id")

    // Check the Validator instance for any errors and use the failedValidationResponse()
    // helper to send the client a response if necessary. 
    if !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Dump the contents of the input struct in a HTTP response.
    fmt.Fprintf(w, "%+v\n", input)
}
```

Create the GET /v1/movies route in our cmd/api/routes.go
```go
    // Add the route for the GET /v1/movies endpoint.
    router.HandlerFunc(http.MethodGet, "/v1/movies", app.listMoviesHandler)
```

> When using curl to send a request containing more than one query string parameter, you must wrap the URL in quotes for it to work correctly.

```shell
$ curl "localhost:4000/v1/movies?title=godfather&genres=crime,drama&page=1&page_size=5&sort=year"
{Title:godfather Genres:[crime drama] Page:1 PageSize:5 Sort:year}
```

### Creating a Filters struct
The page, page_size and sort query string parameters are things that you’ll potentially want to use on other endpoints in your API too. So, to help make this easier, let’s quickly split them out into a reusable Filters struct.

File: internal/data/filters.go
```go
package data

type Filters struct {
    Page     int
    PageSize int
    Sort     string
}
```

Once that’s done, head back to your listMoviesHandler and update it to use the new Filters struct like so:

```go
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
    // Embed the new Filters struct.
    var input struct {
        Title  string
        Genres []string
        data.Filters
    }

    v := validator.New()

    qs := r.URL.Query()

    input.Title = app.readString(qs, "title", "")
    input.Genres = app.readCSV(qs, "genres", []string{})

    // Read the page and page_size query string values into the embedded struct.
    input.Filters.Page = app.readInt(qs, "page", 1, v)
    input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
    
    // Read the sort query string value into the embedded struct.
    input.Filters.Sort = app.readString(qs, "sort", "id")

    if !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    fmt.Fprintf(w, "%+v\n", input)
}
```

## Validating Query String Parameters
We still need to perform some additional sanity checks on the query string values provided by the client. In particular, we want to check that:

- The page value is between 1 and 10,000,000.
- The page_size value is between 1 and 100.
- The sort parameter contains a known and supported value for our movies table. Specifically, we’ll allow "id", "title", "year", "runtime", "-id", "-title", "-year" or "-runtime".

internal/data/filters.go
```go
// Add a SortSafelist field to hold the supported sort values.
type Filters struct {
    Page         int
    PageSize     int
    Sort         string
    SortSafelist []string
}

func ValidateFilters(v *validator.Validator, f Filters) {
    // Check that the page and page_size parameters contain sensible values.
    v.Check(f.Page > 0, "page", "must be greater than zero")
    v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10 million")
    v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
    v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

    // Check that the sort parameter matches a value in the safelist.
    v.Check(validator.PermittedValue(f.Sort, f.SortSafelist...), "sort", "invalid sort value")
}
```

update our listMoviesHandler to set the supported values in the SortSafelist field, and subsequently call this new ValidateFilters()
cmd/api/movies.go
```go
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title  string
        Genres []string
        data.Filters
    }

    v := validator.New()

    qs := r.URL.Query()

    input.Title = app.readString(qs, "title", "")
    input.Genres = app.readCSV(qs, "genres", []string{})

    input.Filters.Page = app.readInt(qs, "page", 1, v)
    input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)

    input.Filters.Sort = app.readString(qs, "sort", "id")
    // Add the supported sort values for this endpoint to the sort safelist.
    input.Filters.SortSafelist = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

    // Execute the validation checks on the Filters struct and send a response 
    // containing the errors if necessary.
    if data.ValidateFilters(v, input.Filters); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    fmt.Fprintf(w, "%+v\n", input)
}
```

```shell
$ curl "localhost:4000/v1/movies?page=-1&page_size=-1&sort=foo"
{
    "error": {
        "page": "must be greater than zero",
        "page_size": "must be greater than zero",
        "sort": "invalid sort value"
    }
}
```

## Listing Data
move on and get our GET /v1/movies endpoint returning some real data.
```go
{
    "movies": [
        {
            "id": 1,
            "title": "Moana",
            "year": 2015,
            "runtime": "107 mins",
            "genres": [
                "animation",
                "adventure"
            ],
            "version": 1
        },
        {
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
        },
        ... etc.
    ]
}
```

### Updating the application
GetAll() method on our database model which executes the following SQL query:
```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
ORDER BY id
```

Because we’re expecting this SQL query to return multiple records, we’ll need to run it using Go’s QueryContext()
```go
// Create a new GetAll() method which returns a slice of movies. Although we're not 
// using them right now, we've set this up to accept the various filter parameters as 
// arguments.
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {
    // Construct the SQL query to retrieve all movie records.
    query := `
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        ORDER BY id`

    // Create a context with a 3-second timeout.
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    // Use QueryContext() to execute the query. This returns a sql.Rows resultset 
    // containing the result.
    rows, err := m.DB.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }

    // Importantly, defer a call to rows.Close() to ensure that the resultset is closed
    // before GetAll() returns.
    defer rows.Close()

    // Initialize an empty slice to hold the movie data.
    movies := []*Movie{}

    // Use rows.Next to iterate through the rows in the resultset.
    for rows.Next() {
        // Initialize an empty Movie struct to hold the data for an individual movie.
        var movie Movie

        // Scan the values from the row into the Movie struct. Again, note that we're 
        // using the pq.Array() adapter on the genres field here.
        err := rows.Scan(
            &movie.ID,
            &movie.CreatedAt,
            &movie.Title,
            &movie.Year,
            &movie.Runtime,
            pq.Array(&movie.Genres),
            &movie.Version,
        )
        if err != nil {
            return nil, err
        }

        // Add the Movie struct to the slice.
        movies = append(movies, &movie)
    }

    // When the rows.Next() loop has finished, call rows.Err() to retrieve any error 
    // that was encountered during the iteration.
    if err = rows.Err(); err != nil {
        return nil, err
    }

    // If everything went OK, then return the slice of movies.
    return movies, nil
}
```

Next up, we need to adapt the listMoviesHandler so that it calls the new GetAll() method to retrieve the movie data, and then writes this data as a JSON response.

```go
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title  string
        Genres []string
        data.Filters
    }

    v := validator.New()

    qs := r.URL.Query()

    input.Title = app.readString(qs, "title", "")
    input.Genres = app.readCSV(qs, "genres", []string{})

    input.Filters.Page = app.readInt(qs, "page", 1, v)
    input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)

    input.Filters.Sort = app.readString(qs, "sort", "id")
    input.Filters.SortSafelist = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

    if data.ValidateFilters(v, input.Filters); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Call the GetAll() method to retrieve the movies, passing in the various filter 
    // parameters.
    movies, err := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }
    
    // Send a JSON response containing the movie data.
    err = app.writeJSON(w, http.StatusOK, envelope{"movies": movies}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

## Filtering Lists
Start putting query string parameters to use  
We’ll build a reductive filter which allows clients to search based on a case-insensitive exact match for movie title and/or one or more movie genres.

```
// List all movies.
/v1/movies

// List movies where the title is a case-insensitive exact match for 'black panther'.
/v1/movies?title=black+panther

// List movies where the genres includes 'adventure'.
/v1/movies?genres=adventure

// List movies where the title is a case-insensitive exact match for 'moana' AND the 
// genres include both 'animation' AND 'adventure'.
/v1/movies?title=moana&genres=animation,adventure
```

### Dynamic filtering in the SQL query
The hardest part of building a dynamic filtering feature like this is the SQL query to retrieve the data — we need it to work with no filters, filters on both title and genres, or a filter on only one of them.

To deal with this, one option is to build up the SQL query dynamically at runtime… with the necessary SQL for each filter concatenated or interpolated into the WHERE clause. But this approach can make your code messy and difficult to understand, especially for large queries which need to support lots of filter options.

In this book we’ll opt for a different technique and use a fixed SQL query which looks like this:

```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (LOWER(title) = LOWER($1) OR $1 = '') 
AND (genres @> $2 OR $2 = '{}') 
ORDER BY id
```

This SQL query is designed so that each of the filters behaves like it is ‘optional’. For example, the condition `(LOWER(title) = LOWER($1) OR $1 = '')` will evaluate as true if the placeholder parameter $1 is a case-insensitive match for the movie title or the placeholder parameter equals ''. So this filter condition will essentially be ‘skipped’ when movie title being searched for is the empty string "".

The `(genres @> $2 OR $2 = '{}')` condition works in the same way. The @> symbol is the ‘contains’ operator for PostgreSQL arrays, and this condition will return true if each value in the placeholder parameter $2 appears in the database genres field or the placeholder parameter contains an empty array.

So, putting this all together, it means that if a client doesn’t provide a title parameter in their query string, then value for the $1 placeholder will be the empty string "", and the filter condition in the SQL query will evaluate to true and act like it has been ‘skipped’. Likewise with the genres parameter.

```go
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {
    // Update the SQL query to include the filter conditions.
    query := `
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE (LOWER(title) = LOWER($1) OR $1 = '') 
        AND (genres @> $2 OR $2 = '{}')     
        ORDER BY id`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    // Pass the title and genres as the placeholder parameter values.
    rows, err := m.DB.QueryContext(ctx, query, title, pq.Array(genres))
    if err != nil {
        return nil, err
    }

    defer rows.Close()

    movies := []*Movie{}

    for rows.Next() {
        var movie Movie

        err := rows.Scan(
            &movie.ID,
            &movie.CreatedAt,
            &movie.Title,
            &movie.Year,
            &movie.Runtime,
            pq.Array(&movie.Genres),
            &movie.Version,
        )
        if err != nil {
            return nil, err
        }

        movies = append(movies, &movie)
    }

    if err = rows.Err(); err != nil {
        return nil, err
    }

    return movies, nil
}
```

## Full-Text Search
In this chapter we’re going to make our movie title filter easier to use by adapting it to support partial matches. For example, if a client wants to find The Breakfast Club they will be able to find it with just the query string title=breakfast.

To implement a basic full-text search on our title field, we’re going to update our SQL query to look like this:
```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
AND (genres @> $2 OR $2 = '{}')     
ORDER BY id
```

The `to_tsvector('simple', title)` function takes a movie title and splits it into lexemes. We specify the simple configuration, which means that the lexemes are just lowercase versions of the words in the title. For example, the movie title "The Breakfast Club" would be split into the lexemes 'breakfast' 'club' 'the'.

The `plainto_tsquery('simple', $1)` function takes a search value and turns it into a formatted query term that PostgreSQL full-text search can understand. It normalizes the search value (again using the simple configuration), strips any special characters, and inserts the and operator & between the words. As an example, the search value "The Club" would result in the query term 'the' & 'club'.

The `@@` operator is the matches operator. In our statement we are using it to check whether the generated query term matches the lexemes. To continue the example, the query term 'the' & 'club' will match rows which contain both lexemes 'the' and 'club'.

// I was considering wildcards, then found out that this is way faster cuz of index. 

```go
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {
    // Use full-text search for the title filter.
    query := `
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '') 
        AND (genres @> $2 OR $2 = '{}')     
        ORDER BY id`

    // Nothing else below needs to change.
    ...
}
```

### Adding indexes
To keep our SQL query performing quickly as the dataset grows, it’s sensible to use indexes to help avoid full table scans and avoid generating the lexemes for the title field every time the query is run.

In our case it makes sense to create GIN indexes on both the genres field and the lexemes generated by to_tsvector(), both which are used in the WHERE clause of our SQL query.

A GIN (Generalized Inverted Index) is a type of index in PostgreSQL designed to handle complex data types and queries efficiently. It is particularly useful for searching within composite values (e.g., arrays, JSONB, full-text search) where a single column may contain multiple elements that need to be queried.

// well, your case, mine's using join table (my mistake)

If you’re following along, go ahead and create a new pair of migration files:
```shell
$ migrate create -seq -ext .sql -dir ./migrations add_movies_indexes
```

### Non-simple configuration
You can also use a language-specific configuration for full-text searches instead of the simple

When you create lexemes or query terms using a language-specific configuration, it will strip out common words for the language and perform word stemming.

So, for example, if you use the english configuration, then the lexemes generated for "One Flew Over the Cuckoo's Nest" would be 'cuckoo' 'flew' 'nest' 'one'. Or with the spanish configuration, the lexemes for "Los lunes al sol" would be 'lun' 'sol'.

You can retrieve a list of all available configurations by running the \dF meta-command in PostgreSQL

English example
```shell
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (to_tsvector('english', title) @@ plainto_tsquery('english', $1) OR $1 = '') 
AND (genres @> $2 OR $2 = '{}')     
ORDER BY id
```

### Using STRPOS and ILIKE
The PostgreSQL `STRPOS()` function allows you to check for the existence of a substring in a particular database field. 
```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (STRPOS(LOWER(title), LOWER($1)) > 0 OR $1 = '') 
AND (genres @> $2 OR $2 = '{}')     
ORDER BY id
```

- From a client perspective, the downside of this is that it may return some unintuitive results. For example, searching for title=the would return both The Breakfast Club and Black Panther in our dataset.
- From a server perspective it’s also not ideal for large datasets. Because there’s no effective way to index the title field to see if the STRPOS()condition is met, it means the query could potentially require a full-table scan each time it is run.

Another option is the `ILIKE` operator, which allows you to find rows which match a specific (case-insensitive) pattern.
```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (title ILIKE $1 OR $1 = '') 
AND (genres @> $2 OR $2 = '{}')     
ORDER BY id
```

This approach would be better from a server point of view because it’s possible to create an index on the title field using the pg_trgm extension and a GIN index

## Sorting Lists
We want to let the client control the sort order via a query string parameter in the format sort={-}{field_name}, where the optional - character is used to indicate a descending sort order.
```shell
// Sort the movies on the title field in ascending alphabetical order.
/v1/movies?sort=title

// Sort the movies on the year field in descending numerical order.
/v1/movies?sort=-year
```

Behind the scenes we will want to translate this into an ORDER BY clause in our SQL query, so that a query string parameter like sort=-year would result in a SQL query like this:
```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (STRPOS(LOWER(title), LOWER($1)) > 0 OR $1 = '')
AND (genres @> $2 OR $2 = '{}')     
ORDER BY year DESC -- <-- Order the result by descending year
```

The difficulty here is that the values for the ORDER BY clause will need to be generated at runtime based on the query string values from the client. 

> Unfortunately it’s not possible to use placeholder parameters for column names or SQL keywords (including ASC and DESC).

So instead, we’ll need to interpolate these dynamic values into our query using fmt.Sprintf() — making sure that the values are checked against a strict safelist first to prevent a SQL injection attack.

In our database multiple movies will have the same year value. If we order based on the year column, then the movies are guaranteed be ordered by year, but the movies for a particular year could appear in any order at any time.

This point is particularly important in the context of an endpoint which provides pagination. We need to make sure that the order of movies is perfectly consistent between requests to prevent items in the list ‘jumping’ between the pages.

Fortunately, guaranteeing the order is simple — we just need to ensure that the ORDER BY clause always includes a primary key column (or another column with a unique constraint on it).

```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (STRPOS(LOWER(title), LOWER($1)) > 0 OR $1 = '') 
AND (genres @> $2 OR $2 = '{}')     
ORDER BY year DESC, id ASC
```

### Implementing sorting
Begin by updating our Filters struct to include some sortColumn() and sortDirection() helpers that transform a query string value (like -year) into values we can use in our SQL query.
```go
// Check that the client-provided Sort field matches one of the entries in our safelist
// and if it does, extract the column name from the Sort field by stripping the leading
// hyphen character (if one exists).
func (f Filters) sortColumn() string {
    for _, safeValue := range f.SortSafelist {
        if f.Sort == safeValue {
            return strings.TrimPrefix(f.Sort, "-")
        }
    }

    panic("unsafe sort parameter: " + f.Sort)
}

// Return the sort direction ("ASC" or "DESC") depending on the prefix character of the
// Sort field.
func (f Filters) sortDirection() string {
    if strings.HasPrefix(f.Sort, "-") {
        return "DESC"
    }

    return "ASC"
}
```

`sortColumn()` function is constructed in such a way that it will panic if the client-provided Sort value doesn’t match one of the entries in our safelist. In theory this shouldn’t happen — the Sort value should have already been checked by calling the ValidateFilters() function — but this is a sensible failsafe to help stop a SQL injection attack occurring.

```go
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {
    // Add an ORDER BY clause and interpolate the sort column and direction. Importantly
    // notice that we also include a secondary sort on the movie ID to ensure a
    // consistent ordering.
    query := fmt.Sprintf(`
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '') 
        AND (genres @> $2 OR $2 = '{}')     
        ORDER BY %s %s, id ASC`, filters.sortColumn(), filters.sortDirection())

    // Nothing else below needs to change.
    ...
}
```

## Paginating Lists
If you have an endpoint which returns a list with hundreds or thousands of records, then for performance or usability reasons you might want to implement some form of pagination on the endpoint — so that it only returns a subset of the records in a single HTTP response.
```shell
// Return the 5 records on page 1 (records 1-5 in the dataset)
/v1/movies?page=1&page_size=5

// Return the next 5 records on page 2 (records 6-10 in the dataset)
/v1/movies?page=2&page_size=5

// Return the next 5 records on page 3 (records 11-15 in the dataset)
/v1/movies?page=3&page_size=5
```

### The LIMIT and OFFSET clauses
Behind the scenes, the simplest way to support this style of pagination is by adding LIMIT and OFFSET clauses to our SQL query.

The LIMIT clause allows you to set the maximum number of records that a SQL query should return, and OFFSET allows you to ‘skip’ a specific number of rows before starting to return records from the query.
```shell
LIMIT = page_size
OFFSET = (page - 1) * page_size
```

```shell
/v1/movies?page_size=5&page=3
```

We would need to ‘translate’ this into the following SQL query:
```sql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
AND (genres @> $2 OR $2 = '{}')     
ORDER BY %s %s, id ASC
LIMIT 5 OFFSET 10
```

```go
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {
    // Update the SQL query to include the LIMIT and OFFSET clauses with placeholder
    // parameter values.
    query := fmt.Sprintf(`
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '') 
        AND (genres @> $2 OR $2 = '{}')     
        ORDER BY %s %s, id ASC
        LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    // As our SQL query now has quite a few placeholder parameters, let's collect the
    // values for the placeholders in a slice. Notice here how we call the limit() and
    // offset() methods on the Filters struct to get the appropriate values for the
    // LIMIT and OFFSET clauses.
    args := []any{title, pq.Array(genres), filters.limit(), filters.offset()}

    // And then pass the args slice to QueryContext() as a variadic parameter.
    rows, err := m.DB.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }

    // Nothing else below needs to change.
    ...
}
```

## Returning Pagination Metadata
It would be even better if we could include some additional metadata along with the response. Information like the current and last page numbers, and the total number of available records would help to give the client context.
```shell
{
    "metadata": {
        "current_page": 1,
        "page_size": 20,
        "first_page": 1,
        "last_page": 42,
        "total_records": 832
    },
    "movies": [
        {
            "id": 1,
            "title": "Moana",
            "year": 2015,
            "runtime": "107 mins",
            "genres": [
                "animation",
                "adventure"
            ],
            "version": 1
        },
        ...
    ]
}
```

### Calculating the total records
The challenging part of doing this is generating the total_records figure. We want this to reflect the total number of available records given the title and genres filters that are applied — not the absolute total of records in the movies table.

A neat way to do this is to adapt our existing SQL query to include a window function which counts the total number of filtered rows, like so:
```sql
SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
AND (genres @> $2 OR $2 = '{}')     
ORDER BY %s %s, id ASC
LIMIT $3 OFFSET $4
```

When PostgreSQL executes this SQL query, the (very simplified) sequence of events runs broadly like this:

- The WHERE clause is used to filter the data in the movies table and get the qualifying rows.
- The window function count(*) OVER() is applied, which counts all the qualifying rows.
- The ORDER BY rules are applied and the qualifying rows are sorted.
- The LIMIT and OFFSET rules are applied and the appropriate sub-set of sorted qualifying rows is returned.

// we cant just use len() the array, this count is pre-limit ._.

### Metadata
File: internal/data/filters.go
```go
// Define a new Metadata struct for holding the pagination metadata.
type Metadata struct {
    CurrentPage  int `json:"current_page,omitempty"`
    PageSize     int `json:"page_size,omitempty"`
    FirstPage    int `json:"first_page,omitempty"`
    LastPage     int `json:"last_page,omitempty"`
    TotalRecords int `json:"total_records,omitempty"`
}

// The calculateMetadata() function calculates the appropriate pagination metadata 
// values given the total number of records, current page, and page size values. Note 
// that when the last page value is calculated we are dividing two int values, and 
// when dividing integer types in Go the result will also be an integer type, with 
// the modulus (or remainder) dropped. So, for example, if there were 12 records in total 
// and a page size of 5, the last page value would be (12+5-1)/5 = 3.2, which is then
// truncated to 3 by Go. 
func calculateMetadata(totalRecords, page, pageSize int) Metadata {
    if totalRecords == 0 {
        // Note that we return an empty Metadata struct if there are no records.
        return Metadata{}
    }

    return Metadata{
        CurrentPage:  page,
        PageSize:     pageSize,
        FirstPage:    1,
        LastPage:     (totalRecords + pageSize - 1) / pageSize,
        TotalRecords: totalRecords,
    }
}
```

Update the GetAll()
```go
// Update the function signature to return a Metadata struct.
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, Metadata, error) {
    // Update the SQL query to include the window function which counts the total 
    // (filtered) records.
    query := fmt.Sprintf(`
        SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '') 
        AND (genres @> $2 OR $2 = '{}')     
        ORDER BY %s %s, id ASC
        LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    args := []any{title, pq.Array(genres), filters.limit(), filters.offset()}

    rows, err := m.DB.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, Metadata{}, err // Update this to return an empty Metadata struct.
    }

    defer rows.Close()

    // Declare a totalRecords variable.
    totalRecords := 0
    movies := []*Movie{}

    for rows.Next() {
        var movie Movie

        err := rows.Scan(
            &totalRecords, // Scan the count from the window function into totalRecords.
            &movie.ID,
            &movie.CreatedAt,
            &movie.Title,
            &movie.Year,
            &movie.Runtime,
            pq.Array(&movie.Genres),
            &movie.Version,
        )
        if err != nil {
            return nil, Metadata{}, err // Update this to return an empty Metadata struct.
        }

        movies = append(movies, &movie)
    }

    if err = rows.Err(); err != nil {
        return nil, Metadata{}, err // Update this to return an empty Metadata struct.
    }

    // Generate a Metadata struct, passing in the total record count and pagination
    // parameters from the client.
    metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

    // Include the metadata struct when returning.
    return movies, metadata, nil
}
```

Update our listMoviesHandler
```go
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title  string
        Genres []string
        data.Filters
    }

    v := validator.New()

    qs := r.URL.Query()

    input.Title = app.readString(qs, "title", "")
    input.Genres = app.readCSV(qs, "genres", []string{})

    input.Filters.Page = app.readInt(qs, "page", 1, v)
    input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)

    input.Filters.Sort = app.readString(qs, "sort", "id")
    input.Filters.SortSafelist = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

    if data.ValidateFilters(v, input.Filters); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Accept the metadata struct as a return value.
    movies, metadata, err := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    // Include the metadata in the response envelope.
    err = app.writeJSON(w, http.StatusOK, envelope{"movies": movies, "metadata": metadata}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

Result
```shell
$ curl "localhost:4000/v1/movies?genres=adventure"
{
    "metadata": {
        "current_page": 1,
        "page_size": 20,
        "first_page": 1,
        "last_page": 1,
        "total_records": 2
    },
    "movies": [
        {
            "id": 1,
            "title": "Moana",
            "year": 2015,
            "runtime": "107 mins",
            "genres": [
                "animation",
                "adventure"
            ],
            "version": 1
        },
        {
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
    ]
}
```

The client now has a lot of control over what their response contains, with filtering, pagination and sorting all supported.
