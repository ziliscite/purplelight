# Chapter 16. Permission-based Authorization
By the time a request leaves our authenticate() middleware, there are now two possible states for the request context. Either:

- The request context contains a User struct (representing a valid, authenticated, user).
- Or the request context contains an AnonymousUser struct.

In this section of the book, we’re going to take this to the next natural stage and look at how to perform different authorization checks to restrict access to our API endpoints.

- Add checks so that only activated users are able to access the various /v1/movies** endpoints.
- Implement a permission-based authorization pattern, which provides fine-grained control over exactly which users can access which endpoints.

## Requiring User Activation
The first thing we’re going to do in terms of authorization is restrict access to our /v1/movies** endpoints — so that they can only be accessed by users who are authenticated (not anonymous), and who have activated their account.

// Why not just, don't let unactivated account be authenticated?

- If the user is anonymous we should send a 401 Unauthorized response and an error message saying “you must be authenticated to access this resource”.
- If the user is not anonymous (i.e. they have authenticated successfully and we know who they are), but they are not activated we should send a 403 Forbidden response and an error message saying “your user account must be activated to access this resource”.
 
> A 401 Unauthorized response should be used when you have missing or bad authentication, and a 403 Forbidden response should be used afterwards, when the user is authenticated but isn’t allowed to perform the requested operation.

```go
func (app *application) authenticationRequiredResponse(w http.ResponseWriter, r *http.Request) {
    message := "you must be authenticated to access this resource"
    app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) inactiveAccountResponse(w http.ResponseWriter, r *http.Request) {
    message := "your user account must be activated to access this resource"
    app.errorResponse(w, r, http.StatusForbidden, message)
}
```

Create the new requireActivatedUser() middleware for carrying out the checks.
```go
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Use the contextGetUser() helper that we made earlier to retrieve the user 
        // information from the request context.
        user := app.contextGetUser(r)

        // If the user is anonymous, then call the authenticationRequiredResponse() to 
        // inform the client that they should authenticate before trying again.
        if user.IsAnonymous() {
            app.authenticationRequiredResponse(w, r)
            return
        }

        // If the user is not activated, use the inactiveAccountResponse() helper to 
        // inform them that they need to activate their account.
        if !user.Activated {
            app.inactiveAccountResponse(w, r)
            return
        }

        // Call the next handler in the chain.
        next.ServeHTTP(w, r)
    })
}
```

Notice here that our requireActivatedUser() middleware has a slightly different signature to the other middleware we’ve built in this book. Instead of accepting and returning a http.Handler, it accepts and returns a http.HandlerFunc.

This is a small change, but it makes it possible to wrap our /v1/movie** handler functions directly with this middleware, without needing to make any further conversions.

File: cmd/api/routes.go
```go
func (app *application) routes() http.Handler {
    router := httprouter.New()

    router.NotFound = http.HandlerFunc(app.notFoundResponse)
    router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

    router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

    // Use the requireActivatedUser() middleware on our five /v1/movies** endpoints.
    router.HandlerFunc(http.MethodGet, "/v1/movies", app.requireActivatedUser(app.listMoviesHandler))
    router.HandlerFunc(http.MethodPost, "/v1/movies", app.requireActivatedUser(app.createMovieHandler))
    router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requireActivatedUser(app.showMovieHandler))
    router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requireActivatedUser(app.updateMovieHandler))
    router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requireActivatedUser(app.deleteMovieHandler))

    router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
    router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)

    router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)

    return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
```

### Demonstration
```shell
$ curl -i localhost:4000/v1/movies/1
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Vary: Authorization
Www-Authenticate: Bearer
Date: Fri, 16 Apr 2021 15:59:33 GMT
Content-Length: 66

{
    "error": "you must be authenticated to access this resource"
}
```

Unactivated account
```shell
$ BODY='{"email": "alice@example.com", "password": "pa55word"}'
$ curl -d "$BODY" localhost:4000/v1/tokens/authentication
{
    "authentication_token": {
        "token": "2O4YHHWDHVVWWDNKN2UZR722BU",
        "expiry": "2021-04-17T18:03:09.598843181+02:00"
    }
}

$ curl -i -H "Authorization: Bearer 2O4YHHWDHVVWWDNKN2UZR722BU" localhost:4000/v1/movies/1
HTTP/1.1 403 Forbidden
Content-Type: application/json
Vary: Authorization
Date: Fri, 16 Apr 2021 16:03:45 GMT
Content-Length: 76

{
    "error": "your user account must be activated to access this resource"
}
```

### Splitting up the middleware
At the moment we have one piece of middleware doing two checks: first it checks that the user is authenticated (not anonymous), and second it checks that they are activated.

But it’s possible to imagine a scenario where you only want to check that a user is authenticated, and you don’t care whether they are activated or not.

To assist with this, you might want to introduce an additional requireAuthenticatedUser() middleware as well as the current requireActivatedUser() middleware.

cmd/api/middleware.go
```go
// Create a new requireAuthenticatedUser() middleware to check that a user is not 
// anonymous.
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := app.contextGetUser(r)

        if user.IsAnonymous() {
            app.authenticationRequiredResponse(w, r)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// Checks that a user is both authenticated and activated.
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
    // Rather than returning this http.HandlerFunc we assign it to the variable fn.
    fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := app.contextGetUser(r)

        // Check that a user is activated.
        if !user.Activated {
            app.inactiveAccountResponse(w, r)
            return
        }

        next.ServeHTTP(w, r)
    })

    // Wrap fn with the requireAuthenticatedUser() middleware before returning it.
    return app.requireAuthenticatedUser(fn)
}
```

The way that we’ve set this up, our requireActivatedUser() middleware now automatically calls the requireAuthenticatedUser() middleware before being executed itself.

## Setting up the Permissions Database Table
Restricting our API so that movie data can only be accessed and edited by activated users is useful, but sometimes you might need a more granular level of control. For example, in our case we might be happy for ‘regular’ users of our API to read the movie data (so long as they are activated), but we want to restrict write access to a smaller subset of trusted users.

We’re going to introduce the concept of permissions to our application.

| Method | URL Pattern | Required permission |
| --- | --- | --- |
| GET | /v1/healthcheck | — |
| GET | /v1/movies | movies:read |
| POST | /v1/movies | movies:write |
| GET | /v1/movies/:id | movies:read |
| PATCH | /v1/movies/:id | movies:write |
| DELETE | /v1/movies/:id | movies:write |
| POST | /v1/users | — |
| PUT | /v1/users/activated | — |
| POST | /v1/tokens/authentication | — |


### Relationship between permissions and users
The relationship between permissions and users is a great example of a many-to-many relationship. One user may have many permissions, and the same permission may belong to many users.

Let’s say that we are storing our user data in a users table which looks like this:
```
id	email	…
1	alice@example.com	…
2	bob@example.com	…
```

And our permissions data is stored in a permissions table like this:
```
id	code
1	movies:read
2	movies:write
```

Then we can create a joining table called users_permissions to store the information about which users have which permissions, similar to this:
```
user_id	permission_id
1	1
2	1
2	2

```

In the example above, the user alice@example.com (user ID 1) has the movies:read (permission ID 1) permission only, whereas bob@example.com (user ID 2) has both the movies:read and movies:write permissions.

// Guess what? I already implemented this with anime and tags

We might wanna have this
`PermissionModel.GetAllForUser(user)       → Retrieve all permissions for a user
UserModel.GetAllForPermission(permission) → Retrieve all users with a specific permission`

### Creating the SQL migrations
```shell
migrate create -seq -ext .sql -dir ./migrations add_permissions
```

add_permissions.up.sql
```sql
CREATE TABLE IF NOT EXISTS permissions (
    id bigserial PRIMARY KEY,
    code text NOT NULL
);

CREATE TABLE IF NOT EXISTS users_permissions (
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
    permission_id bigint NOT NULL REFERENCES permissions ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);

-- Add the two permissions to the table.
INSERT INTO permissions (code)
VALUES 
('movies:read'),
('movies:write');
```

- The PRIMARY KEY (user_id, permission_id) line sets a composite primary key on our users_permissions table, where the primary key is made up of both the users_id and permission_id columns. Setting this as the primary key essentially means that the same user/permission combination can only appear once in the table and cannot be duplicated.

- When creating the users_permissions table we use the REFERENCES user syntax to create a foreign key constraint against the primary key of our users table, which ensures that any value in the user_id column has a corresponding entry in our users table. And likewise, we use the REFERENCES permissions syntax to ensure that the permission_id column has a corresponding entry in the permissions table.

add_permissions.down.sql
```sql
DROP TABLE IF EXISTS users_permissions;
DROP TABLE IF EXISTS permissions;
```

migrate
```shell
$ migrate -path ./migrations -database $PURPLELIGHT_DSN up
```

## Setting up the Permissions Model
We want to include in this model is a GetAllForUser() method to return all permission codes for a specific user. 

The idea is that we’ll be able to use this in our handlers and middleware like so:
```go
// Return a slice of the permission codes for the user with ID = 1. This would return
// something like []string{"movies:read", "movies:write"}.
app.models.Permissions.GetAllForUser(1) 
```

Behind the scenes, the SQL statement that we need to fetch the permission codes for a specific user looks like this:
```sql
SELECT permissions.code
FROM permissions
INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
INNER JOIN users ON users_permissions.user_id = users.id
WHERE users.id = $1
```

In this query we are using the INNER JOIN clause to join our permissions table to our users_permissions table, and then using it again to join that to the users table.
```go
// Define a Permissions slice, which we will use to hold the permission codes (like
// "movies:read" and "movies:write") for a single user.
type Permissions []string

// Add a helper method to check whether the Permissions slice contains a specific 
// permission code.
func (p Permissions) Include(code string) bool {
    return slices.Contains(p, code)
}
```

```go
// Define the PermissionModel type.
type PermissionModel struct {
    DB *sql.DB
}

// The GetAllForUser() method returns all permission codes for a specific user in a 
// Permissions slice. The code in this method should feel very familiar --- it uses the
// standard pattern that we've already seen before for retrieving multiple data rows in 
// an SQL query.
func (m PermissionModel) GetAllForUser(userID int64) (Permissions, error) {
    query := `
        SELECT permissions.code
        FROM permissions
        INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
        INNER JOIN users ON users_permissions.user_id = users.id
        WHERE users.id = $1`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    rows, err := m.DB.QueryContext(ctx, query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var permissions Permissions

    for rows.Next() {
        var permission string

        err := rows.Scan(&permission)
        if err != nil {
            return nil, err
        }

        permissions = append(permissions, permission)
    }
    if err = rows.Err(); err != nil {
        return nil, err
    }

    return permissions, nil
}
```

## Checking Permissions
Conceptually, what we need to do here isn’t too complicated.

- We’ll make a new requirePermission() middleware which accepts a specific permission code like "movies:read" as an argument.
- In this middleware we’ll retrieve the current user from the request context, and call the app.models.Permissions.GetAllForUser() method (which we just made) to get a slice of their permissions.
- Then we can check to see if the slice contains the specific permission code needed. If it doesn’t, we should send the client a 403 Forbidden response.

To put this into practice, let’s first make a new notPermittedResponse()
```go
func (app *application) notPermittedResponse(w http.ResponseWriter, r *http.Request) {
    message := "your user account doesn't have the necessary permissions to access this resource"
    app.errorResponse(w, r, http.StatusForbidden, message)
}
```

The new `requirePermission()` middleware method we’re going to set up automatically wraps our existing `requireActivatedUser()` middleware, which in turn, wraps our `requireAuthenticatedUser()` middleware.

Which ensure that the request is from an authenticated (non-anonymous), activated user, who has a specific permission.

```go
// Note that the first parameter for the middleware function is the permission code that
// we require the user to have.
func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
    fn := func(w http.ResponseWriter, r *http.Request) {
        // Retrieve the user from the request context.
        user := app.contextGetUser(r)

        // Get the slice of permissions for the user.
        permissions, err := app.models.Permissions.GetAllForUser(user.ID)
        if err != nil {
            app.serverErrorResponse(w, r, err)
            return
        }

        // Check if the slice includes the required permission. If it doesn't, then 
        // return a 403 Forbidden response.
        if !permissions.Include(code) {
            app.notPermittedResponse(w, r)
            return
        }

        // Otherwise they have the required permission so we call the next handler in
        // the chain.
        next.ServeHTTP(w, r)
    }

    // Wrap this with the requireActivatedUser() middleware before returning it.
    return app.requireActivatedUser(fn)
}
```

Update the routes so that our API requires the "movies:read" permission for the endpoints that fetch movie data, and the "movies:write" permission for the endpoints that create, edit or delete a movie.
```go
func (app *application) routes() http.Handler {
    router := httprouter.New()

    router.NotFound = http.HandlerFunc(app.notFoundResponse)
    router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

    router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

    // Use the requirePermission() middleware on each of the /v1/movies** endpoints,
    // passing in the required permission code as the first parameter.
    router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))
    router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission("movies:write", app.createMovieHandler))
    router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission("movies:read", app.showMovieHandler))
    router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission("movies:write", app.updateMovieHandler))
    router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission("movies:write", app.deleteMovieHandler))

    router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
    router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)

    router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)

    return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
```

### Demonstration
Open psql and add some permissions.

- Activate the user alice@example.com.
- Give all users the "movies:read" permission.
- Give the user faith@example.com the "movies:write" permission.

```sql
-- Set the activated field for alice@example.com to true.
UPDATE users SET activated = true WHERE email = 'alice@example.com';

-- Give all users the 'movies:read' permission
INSERT INTO users_permissions
SELECT id, (SELECT id FROM permissions WHERE code = 'movies:read') FROM users;

-- Give faith@example.com the 'movies:write' permission
INSERT INTO users_permissions
VALUES (
    (SELECT id FROM users WHERE email = 'faith@example.com'),
    (SELECT id FROM permissions WHERE  code = 'movies:write')
);

-- List all activated users and their permissions.
SELECT email, array_agg(permissions.code) as permissions
FROM permissions
INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
INNER JOIN users ON users_permissions.user_id = users.id
WHERE users.activated = true
GROUP BY email;
```

Read permissions users
```shell
$ BODY='{"email": "alice@example.com", "password": "pa55word"}'
$ curl -d "$BODY" localhost:4000/v1/tokens/authentication
{
    "authentication_token": {
        "token": "OPFXEPOYZWMGNWXWKMYIMEGATU",
        "expiry": "2021-04-17T20:49:39.963768416+02:00"
    }
}

$ curl -H "Authorization: Bearer OPFXEPOYZWMGNWXWKMYIMEGATU" localhost:4000/v1/movies/1
{
    "movie": {
        "id": 1,
        "title": "Moana",
        "year": 2016,
        "runtime": "107 mins",
        "genres": [
            "animation",
            "adventure"
        ],
        "version": 1
    }
}

$ curl -X DELETE -H "Authorization: Bearer OPFXEPOYZWMGNWXWKMYIMEGATU" localhost:4000/v1/movies/1
{
    "error": "your user account doesn't have the necessary permissions to access this resource"
}
```

Write permissions user
```shell
$ BODY='{"email": "faith@example.com", "password": "pa55word"}'
$ curl -d "$BODY" localhost:4000/v1/tokens/authentication
{
    "authentication_token": {
        "token": "E42XD5OBBBO4MPUPYGLLY2GURE",
        "expiry": "2021-04-17T20:51:14.924813208+02:00"
    }
}

$ curl -X DELETE -H "Authorization: Bearer E42XD5OBBBO4MPUPYGLLY2GURE" localhost:4000/v1/movies/1
{
    "message": "movie successfully deleted"
}
```

## Granting Permissions
At the moment — when a new user registers an account they don’t have any permissions. In this chapter we’re going to change that so that new users are automatically granted the "movies:read" permission by default.

### Updating the permissions model
Include an `AddForUser()` method, which adds one or more permission codes for a specific user to our database.

Handlers finna looks like this
```go
// Add the "movies:read" and "movies:write" permissions for the user with ID = 2.
app.models.Permissions.AddForUser(2, "movies:read", "movies:write")
```

Behind the scenes, the SQL statement that we need to insert this data looks like this:
```sql
INSERT INTO users_permissions
SELECT $1, permissions.id FROM permissions 
WHERE permissions.code = ANY($2)
```

In this query the $1 parameter will be the user’s ID, and the $2 parameter will be a PostgreSQL array of the permission codes that we want to add for the user, like {'movies:read', 'movies:write'}.

So what’s happening here is that the `SELECT ...` statement on the second line creates an ‘interim’ table with rows made up of the user ID `and the corresponding IDs for the permission codes in the array`. Then we insert the contents of this interim table into our user_permissions table.

Example:
If $1 = 123 and $2 = ['edit', 'delete'], the query:

Finds the ids for permissions with codes edit and delete.

Inserts rows like (123, 4) and (123, 7) into users_permissions (assuming edit has id=4, delete has id=7).

```go
// Add the provided permission codes for a specific user. Notice that we're using a 
// variadic parameter for the codes so that we can assign multiple permissions in a 
// single call.
func (m PermissionModel) AddForUser(userID int64, codes ...string) error {
    query := `
        INSERT INTO users_permissions
        SELECT $1, permissions.id FROM permissions WHERE permissions.code = ANY($2)`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    _, err := m.DB.ExecContext(ctx, query, userID, pq.Array(codes))
    return err
}
```

### Updating the registration handler
```go
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Name     string `json:"name"`
        Email    string `json:"email"`
        Password string `json:"password"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    user := &data.User{
        Name:      input.Name,
        Email:     input.Email,
        Activated: false,
    }

    err = user.Password.Set(input.Password)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    v := validator.New()

    if data.ValidateUser(v, user); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    err = app.models.Users.Insert(user)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrDuplicateEmail):
            v.AddError("email", "a user with this email address already exists")
            app.failedValidationResponse(w, r, v.Errors)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // Add the "movies:read" permission for the new user.
    err = app.models.Permissions.AddForUser(user.ID, "movies:read")
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    app.background(func() {
        data := map[string]any{
            "activationToken": token.Plaintext,
            "userID":          user.ID,
        }

        err = app.mailer.Send(user.Email, "user_welcome.tmpl", data)
        if err != nil {
            app.logger.Error(err.Error())
        }
    })

    err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```
