# Chapter 12. User Model Setup and Registration
In the upcoming sections of this book, we’re going to shift our focus towards users: registering them, activating them, authenticating them, and restricting access to our API endpoints depending on the permissions that they have.

But before we can do these things, we need to lay some groundwork. Specifically we need to:

- Create a new users table in PostgreSQL for storing our user data.
- Create a UserModel which contains the code for interacting with our users table, validating user data, and hashing user passwords.
- Develop a POST /v1/users endpoint which can be used to register new users in our application.

## Setting up the Users Database Table
Let’s begin by creating a new users table in our database.
```shell
$ migrate create -seq -ext sql -dir ./migrations create_users_table
C:\Users\manzi\GolandProjects\purplelight\migrations\000003_create_users_table.up.sql
C:\Users\manzi\GolandProjects\purplelight\migrations\000003_create_users_table.down.sql
```

Up
```sql
CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    name text NOT NULL,
    email citext UNIQUE NOT NULL,
    password_hash bytea NOT NULL,
    activated bool NOT NULL,
    version integer NOT NULL DEFAULT 1
);
```

Down
```sql
DROP TABLE IF EXISTS users;
```

There are a few interesting about this CREATE TABLE statement:  
- The email column has the type citext (case-insensitive text). This type stores text data exactly as it is inputted — without changing the case in any way — but comparisons against the data are always case-insensitive… including lookups on associated indexes.
- We’ve also got a UNIQUE constraint on the email column. Combined with the citext type, this means that no two rows in the database can have the same email value — even if they have different cases. This essentially enforces a database-level business rule that `no two users should exist with the same email` address.
- The password_hash column has the type bytea (binary string). In this column we’ll store a one-way hash of the user’s password generated using bcrypt — not the plaintext password itself.
- The activated column stores a boolean value to denote whether a user account is ‘active’ or not. We will set this to false by default when creating a new user, and require the user to confirm their email address before we set it to true.
- We’ve also included a version number column, which we will increment each time a user record is updated. This will allow us to use optimistic locking to prevent race conditions when updating user records, in the same way that we did with movies earlier in the book.

Execute the migration using the following command:
```shell
migrate -path ./migrations -database %PURPLELIGHT_DSN% up
```

One important thing to point out here: the UNIQUE constraint on our email column has automatically been assigned the name users_email_key.

## Setting up the Users Model
We are going to update our internal/data package to contain a new User struct (to represent the data for an individual user), and create a UserModel type (which we will use to perform various SQL queries against our users table).

Start by defining the User struct, along with some helper methods for setting and verifying the password for a user.

The first thing we need to do is install the golang.org/x/crypto/bcrypt package
```shell
go get golang.org/x/crypto/bcrypt@latest
```

In the internal/data/users.go file
```go
// Define a User struct to represent an individual user. Importantly, notice how we are 
// using the json:"-" struct tag to prevent the Password and Version fields appearing in
// any output when we encode it to JSON. Also notice that the Password field uses the
// custom password type defined below.
type User struct {
    ID        int64     `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Password  password  `json:"-"`
    Activated bool      `json:"activated"`
    Version   int       `json:"-"`
}

// Create a custom password type which is a struct containing the plaintext and hashed 
// versions of the password for a user. The plaintext field is a *pointer* to a string,
// so that we're able to distinguish between a plaintext password not being present in 
// the struct at all, versus a plaintext password which is the empty string "".
type password struct {
    plaintext *string
    hash      []byte
}

// The Set() method calculates the bcrypt hash of a plaintext password, and stores both 
// the hash and the plaintext versions in the struct.
func (p *password) Set(plaintextPassword string) error {
    hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
    if err != nil {
        return err
    }

    p.plaintext = &plaintextPassword
    p.hash = hash

    return nil
}

// The Matches() method checks whether the provided plaintext password matches the 
// hashed password stored in the struct, returning true if it matches and false 
// otherwise.
func (p *password) Matches(plaintextPassword string) (bool, error) {
    err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
    if err != nil {
        switch {
        case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
            return false, nil
        default:
            return false, err
        }
    }

    return true, nil
}
```

- The bcrypt.GenerateFromPassword() function generates a bcrypt hash of a password using a specific cost parameter (in the code above, we use a cost of 12). The higher the cost, the slower and more computationally expensive it is to generate the hash. There is a balance to be struck here — we want the cost to be prohibitively expensive for attackers, but also not so slow that it harms the user experience of our API. This function returns a hash string in the format:  
`$2b$[cost]$[22-character salt][31-character hash]`

- The bcrypt.CompareHashAndPassword() function works by re-hashing the provided password using the same salt and cost parameter that is in the hash string that we’re comparing against. The re-hashed value is then checked against the original hash string using the subtle.ConstantTimeCompare() function, which performs a comparison in constant time (to mitigate the risk of a timing attack). If they don’t match, then it will return a bcrypt.ErrMismatchedHashAndPassword error.

### Adding Validation Checks
- Check that the Name field is not the empty string, and the value is less than 500 bytes long.
- Check that the Email field is not the empty string, and that it matches the regular expression for email addresses that we added in our validator package earlier in the book.
- If the Password.plaintext field is not nil, then check that the value is not the empty string and is between 8 and 72 bytes long.
- Check that the Password.hash field is never nil.

> When creating a bcrypt hash the input is truncated to a maximum of 72 bytes. So, if someone uses a very long password, it means that any bytes after that would effectively be ignored when creating the hash.

### Creating the UserModel
Or in my case, a repository

```go
// Define a custom ErrDuplicateEmail error.
var (
    ErrDuplicateEmail = errors.New("duplicate email")
)

...

// Create a UserModel struct which wraps the connection pool.
type UserModel struct {
    DB *sql.DB
}

// Insert a new record in the database for the user. Note that the id, created_at and 
// version fields are all automatically generated by our database, so we use the 
// RETURNING clause to read them into the User struct after the insert, in the same way 
// that we did when creating a movie.
func (m UserModel) Insert(user *User) error {
    query := `
        INSERT INTO users (name, email, password_hash, activated) 
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, version`

    args := []any{user.Name, user.Email, user.Password.hash, user.Activated}

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    // If the table already contains a record with this email address, then when we try 
    // to perform the insert there will be a violation of the UNIQUE "users_email_key" 
    // constraint that we set up in the previous chapter. We check for this error 
    // specifically, and return custom ErrDuplicateEmail error instead.
    err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
    if err != nil {
        switch {
        case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
            return ErrDuplicateEmail
        default:
            return err
        }
    }

    return nil
}

// Retrieve the User details from the database based on the user's email address.
// Because we have a UNIQUE constraint on the email column, this SQL query will only 
// return one record (or none at all, in which case we return a ErrRecordNotFound error).
func (m UserModel) GetByEmail(email string) (*User, error) {
    query := `
        SELECT id, created_at, name, email, password_hash, activated, version
        FROM users
        WHERE email = $1`

    var user User

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    err := m.DB.QueryRowContext(ctx, query, email).Scan(
        &user.ID,
        &user.CreatedAt,
        &user.Name,
        &user.Email,
        &user.Password.hash,
        &user.Activated,
        &user.Version,
    )

    if err != nil {
        switch {
        case errors.Is(err, sql.ErrNoRows):
            return nil, ErrRecordNotFound
        default:
            return nil, err
        }
    }

    return &user, nil
}

// Update the details for a specific user. Notice that we check against the version 
// field to help prevent any race conditions during the request cycle, just like we did
// when updating a movie. And we also check for a violation of the "users_email_key" 
// constraint when performing the update, just like we did when inserting the user 
// record originally.
func (m UserModel) Update(user *User) error {
    query := `
        UPDATE users 
        SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
        WHERE id = $5 AND version = $6
        RETURNING version`

    args := []any{
        user.Name,
        user.Email,
        user.Password.hash,
        user.Activated,
        user.ID,
        user.Version,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
    if err != nil {
        switch {
        case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
            return ErrDuplicateEmail
        case errors.Is(err, sql.ErrNoRows):
            return ErrEditConflict
        default:
            return err
        }
    }

    return nil
}
```

Also the main model
```go
type Models struct {
    Movies MovieModel
    Users  UserModel // Add a new Users field.
}

func NewModels(db *sql.DB) Models {
    return Models{
        Movies: MovieModel{DB: db},
        Users:  UserModel{DB: db}, // Initialize a new UserModel instance.
    }
}
```

## Registering a User
Add a handler
`POST	/v1/users 	registerUserHandler   Register a new user`

When a client calls this new POST /v1/users endpoint, we will expect them to provide the following details for the new user in a JSON request body. Similar to this:

```shell
{
    "name": "Alice Smith",
    "email": "alice@example.com",
    "password": "pa55word"
}
```

When we receive this, the registerUserHandler should create a new User struct containing these details, validate it with the ValidateUser() helper, and then pass it to our UserModel.Insert() method to create a new database record.

```go
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
    // Create an anonymous struct to hold the expected data from the request body.
    var input struct {
        Name     string `json:"name"`
        Email    string `json:"email"`
        Password string `json:"password"`
    }

    // Parse the request body into the anonymous struct.
    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    // Copy the data from the request body into a new User struct. Notice also that we
    // set the Activated field to false, which isn't strictly necessary because the 
    // Activated field will have the zero-value of false by default. But setting this 
    // explicitly helps to make our intentions clear to anyone reading the code.
    user := &data.User{
        Name:      input.Name,
        Email:     input.Email,
        Activated: false,
    }

    // Use the Password.Set() method to generate and store the hashed and plaintext 
    // passwords.
    err = user.Password.Set(input.Password)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    v := validator.New()

    // Validate the user struct and return the error messages to the client if any of 
    // the checks fail.
    if data.ValidateUser(v, user); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Insert the user data into the database.
    err = app.models.Users.Insert(user)
    if err != nil {
        switch {
        // If we get a ErrDuplicateEmail error, use the v.AddError() method to manually
        // add a message to the validator instance, and then call our 
        // failedValidationResponse() helper.
        case errors.Is(err, data.ErrDuplicateEmail):
            v.AddError("email", "a user with this email address already exists")
            app.failedValidationResponse(w, r, v.Errors)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // Write a JSON response containing the user data along with a 201 Created status 
    // code.
    err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

we also need to add it to our routes
```go
// Add the route for the POST /v1/users endpoint.
    router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
```

Example
```shell
$ BODY='{"name": "Alice Smith", "email": "alice@example.com", "password": "pa55word"}'
$ curl -i -d "$BODY" localhost:4000/v1/users
HTTP/1.1 201 Created
Content-Type: application/json
Date: Mon, 15 Mar 2021 14:42:58 GMT
Content-Length: 152

{
    "user": {
        "id": 1,
        "created_at": "2021-03-15T15:42:58+01:00",
        "name": "Alice Smith",
        "email": "alice@example.com",
        "activated": false
    }
}
```

Check the psql
```shell
$ psql $GREENLIGHT_DB_DSN
Password for user greenlight: 
psql (15.4 (Ubuntu 15.4-1.pgdg22.04+1))
SSL connection (protocol: TLSv1.3, cipher: TLS_AES_256_GCM_SHA384, bits: 256, compression: off)
Type "help" for help.

greenlight=> SELECT * FROM users;
 id |       created_at       |    name     |       email       |           password_hash             | activated | version 
----+------------------------+-------------+-------------------+-------------------------------------+-----------+---------
  1 | 2021-04-11 14:29:45+02 | Alice Smith | alice@example.com | \x24326124313224526157784d67356d... | f         |       1
(1 row)
```

> The psql tool always displays bytea values as a hex-encoded string. So the password_hash field in the output above displays a hex-encoding of the bcrypt hash. 

If you want, you can run the following query to append the regular string version to the table too: 
```sql
SELECT *, encode(password_hash, 'escape') FROM users;
```

Invalid requests:
```shell
$ BODY='{"name": "", "email": "bob@invalid.", "password": "pass"}'
$ curl -d "$BODY" localhost:4000/v1/users
{
    "error": {
        "email": "must be a valid email address",
        "name": "must be provided",
        "password": "must be at least 8 bytes long"
    }
}
```

```shell
$ BODY='{"name": "Alice Jones", "email": "alice@example.com", "password": "pa55word"}'
$ curl -i -d "$BODY" localhost:4000/v1/users
HTTP/1.1 422 Unprocessable Entity
Cache-Control: no-store
Content-Type: application/json
Date: Wed, 30 Dec 2020 14:22:06 GMT
Content-Length: 78

{
    "error": {
        "email": "a user with this email address already exists"
    }
}
```

### Email case-sensitivity
- Thanks to the specifications in RFC 2821, the domain part of an email address (username@domain) is case-insensitive. This means we can be confident that the real-life user behind alice@example.com is the same person as alice@EXAMPLE.COM.

- The username part of an email address may or may not be case-sensitive — it depends on the email provider. Almost every major email provider treats the username as case-insensitive, but it is not absolutely guaranteed. All we can say here is that the real-life user behind the address alice@example.com is very probably (but not definitely) the same as ALICE@example.com.

From a security point of view, we should always store the email address using the exact casing provided by the user during registration, and we should send them emails using that exact casing only. If we don’t, there is a risk that emails could be delivered to the wrong real-life user.

### User enumeration
It’s important to be aware that our registration endpoint is vulnerable to user enumeration. For example, if an attacker wants to know whether alice@example.com has an account with us, all they need to do is send a request like this:

```shell
$ BODY='{"name": "Alice Jones", "email": "alice@example.com", "password": "pa55word"}'
$ curl -d "$BODY" localhost:4000/v1/users
{
    "error": {
        "email": "a user with this email address already exists"
    }
}
```

And they have the answer right there. We’re explicitly telling the attacker that alice@example.com is already a user.

So, what are the risks of leaking this information?

The first, most obvious, risk relates to user privacy. For services that are sensitive or confidential you probably don’t want to make it obvious who has an account. The second risk is that it makes it easier for an attacker to compromise a user’s account. Once they know a user’s email address, they can potentially:

- Target the user with social engineering or another type of tailored attack.
- Search for the email address in leaked password tables, and try those same passwords on our service.

Preventing enumeration attacks typically requires two things:

- Making sure that the response sent to the client is always exactly the same, irrespective of whether a user exists or not. Generally, this means changing your response wording to be ambiguous, and notifying the user of any problems in a side-channel (such as sending them an email to inform them that they already have an account).
- Making sure that the time taken to send the response is always the same, irrespective of whether a user exists or not. In Go, this generally means offloading work to a background goroutine.

Unfortunately, these mitigations tend to increase the complexity of your application and add friction and obscurity to your workflows. For all your regular users who are not attackers, they’re a negative from a UX point of view.

It’s worth noting that many big-name services, including Twitter, GitHub and Amazon, don’t prevent user enumeration (at least not on their registration pages).