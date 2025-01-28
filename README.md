# Chapter 15. Authentication
In this section of the book we’re going to look at how to authenticate requests to our API, so that we know exactly which user a particular request is coming from.

> Authentication is about confirming who a user is, whereas authorization is about checking whether that user is permitted to do something.

- Lay out the possible approaches to API authentication that we could use, and talk through their relative pros and cons.
- Implement a stateful token-based authentication pattern, which allows clients to exchange their user credentials for a time-limited authentication token identifying who they are.

## Authentication Options
Specifically, the five approaches that we’ll compare are:

- Basic authentication
- Stateful token authentication
- Stateless token authentication
- API key authentication
- OAuth 2.0 / OpenID Connect

### HTTP basic authentication
The client includes an Authorization header with every request containing their credentials. The credentials need to be in the format username:password and base-64 encoded.

So, for example, to authenticate as alice@example.com:pa55word the client would send the following header:

`Authorization: Basic YWxpY2VAZXhhbXBsZS5jb206cGE1NXdvcmQ=`

We extract the credentials using `Request.BasicAuth` and verify it.

Its very simple for the client, and is supported out of the box by most programming languages, web browsers, and tools such as curl and wget.

For the server, its not very great. Comparing the password provided by a client against a (slow) hashed password is a deliberately costly operation, and when using HTTP basic authentication you need to do that check for every request.

That will create a lot of extra work for your API server and add significant latency to responses.

### Token authentication
The high-level idea behind token authentication (also sometimes known as bearer token authentication) works like this:

1. The client sends a request to your API containing their credentials (typically username or email address, and password).

2. The API verifies that the credentials are correct, generates a bearer token which represents the user, and sends it back to the user. The token expires after a set period of time, after which the user will need to resubmit their credentials again to get a new token.

3. For subsequent requests to the API, the client includes the token in an Authorization header like this:
`Authorization: Bearer <token>`

4. When your API receives this request, it checks that the token hasn’t expired and examines the token value to determine who the user is.

For APIs where user passwords are hashed (like ours), this approach is better because it means that the slow password check only has to be done periodically — either when creating a token for the first time or after a token has expired.

The downside is that managing tokens can be complicated for clients — they will need to implement the necessary logic for caching tokens, monitoring and managing token expiry, and periodically generating new tokens.

### Stateful token authentication
In a stateful token approach, the value of the token is a high-entropy cryptographically-secure random string. This token — or a fast hash of it — is stored server-side in a database, alongside the user ID and an expiry time for the token.

When the client sends back the token in subsequent requests, your application can look up the token in the database, check that it hasn’t expired, and retrieve the corresponding user ID to find out who the request is coming from.

The big advantage of this is that your API maintains control over the tokens — it’s straightforward to revoke tokens on a per-token or per-user basis by deleting them from the database or marking them as expired.

The fact that it requires a database lookup is a negative — but in most cases you will need to make a database lookup to check the user’s activation status or retrieve additional information about them anyway.

### Stateless token authentication
In contrast, stateless tokens encode the user ID and expiry time in the token itself. The token is cryptographically signed to prevent tampering and (in some cases) encrypted to prevent the contents being read.

There are a few different technologies that you can use to create stateless tokens. Encoding the information in a JWT (JSON Web Token) is probably the most well-known approach.

The main selling point of using stateless tokens for authentication is that the work to encode and decode the token can be done in memory, and all the information required to identify the user is contained within the token itself. There’s no need to perform a database lookup to find out who a request is coming from.

The primary downside of stateless tokens is that they can’t easily be revoked once they are issued.

In an emergency, you could effectively revoke all tokens by changing the secret used for signing your tokens (forcing all users to re-authenticate), or another workaround is to maintain a blocklist of revoked tokens in a database (although that defeats the ‘stateless’ aspect of having stateless tokens).

> You should generally avoid storing additional information in a stateless token, such as a user’s activation status or permissions, and using that as the basis for authorization checks. During the lifetime of the token, the information encoded into it will potentially become stale and out-of-sync with the real data in your system — and relying on stale data for authorization checks can easily lead to unexpected behavior for users and various security issues.

They can be very useful in a scenario where you need delegated authentication — where the application creating the authentication token is different to the application consuming it.

For instance, if you’re building a system which has a microservice-style architecture behind the scenes, then a stateless token created by an ‘authentication’ service can subsequently be passed to other services to identify the user.

### API-key authentication
The idea behind API-key authentication is that a user has a non-expiring secret ‘key’ associated with their account. This key should be a high-entropy cryptographically-secure random string, and a fast hash of the key (SHA256 or SHA512) should be stored alongside the corresponding user ID in your database.

`Authorization: Key <key>`

On receiving it, your API can regenerate the fast hash of the key and use it to lookup the corresponding user ID from your database.

Conceptually, this isn’t a million miles away from the stateful token approach — the main difference is that the keys are permanent keys, rather than temporary tokens.

It’s also important to note that API keys themselves should only ever be communicated to users over a secure channel, and you should treat them with the same level of care that you would a user’s password.

### OAuth 2.0 / OpenID Connect
With this approach, information about your users (and their passwords) is stored by a third-party identity provider like Google or Facebook rather than yourself.

> OAuth 2.0 is not an authentication protocol, and you shouldn’t really use it for authenticating users.

If you want to implement authentication checks against a third-party identity provider, you should use OpenID Connect (which is built directly on top of OAuth 2.0).

There’s a comprehensive overview of OpenID Connect [here](https://connect2id.com/learn/openid-connect), but at a very, very, high level it works like this:

- When you want to authenticate a request, you redirect the user to an ‘authentication and consent’ form hosted by the identity provider.
- If the user consents, then the identity provider sends your API an authorization code.
- Your API then sends the authorization code to another endpoint provided by the identity provider. They verify the authorization code, and if it’s valid they will send you a JSON response containing an ID token.
- This ID token is itself a JWT. You need to validate and decode this JWT to get the actual user information, which includes things like their email address, name, birth date, timezone etc.
- Now that you know who the user is, you can then implement a stateful or stateless authentication token pattern so that you don’t have to go through the whole process for every subsequent request.

Like all the other options we’ve looked at, there are pros and cons to using OpenID Connect. The big plus is that you don’t need to persistently store user information or passwords yourself. The big downside is that it’s quite complex — although there are some helper packages like `coreos/go-oidc`.

It’s also important to point out that using OpenID Connect requires all your users to have an account with the identity provider, and the ‘authentication and consent’ step requires human interaction via a web browser — which is probably fine if your API is the back-end for a website, but not ideal if it is a ‘standalone’ API with other computer programs as clients.

### What authentication approach should I use?
A simple, rough, rules-of-thumb:
- If your API doesn’t have ‘real’ user accounts with slow password hashes, then HTTP basic authentication can be a good — and often overlooked — fit.
- If you don’t want to store user passwords yourself, all your users have accounts with a third-party identity provider that supports OpenID Connect, and your API is the back-end for a website… then use OpenID Connect.
- If you require delegated authentication, such as when your API has a microservice architecture with different services for performing authentication and performing other tasks, then use stateless authentication tokens.
- Otherwise use API keys or stateful authentication tokens. In general:
  - Stateful authentication tokens are a nice fit for APIs that act as the back-end for a website or single-page application, as there is a natural moment when the user logs-in where they can be exchanged for user credentials.
  - In contrast, API keys can be better for more ‘general purpose’ APIs because they’re permanent and simpler for developers to use in their applications and scripts.

In the rest of this book, we’re going to implement authentication using the stateful authentication token pattern.

## Generating Authentication Tokens
We’re going to focus on building up the code for a new POST/v1/tokens/authentication endpoint, which will allow a client to exchange their credentials (email address and password) for a stateful authentication token.

- The client sends a JSON request to a new POST/v1/tokens/authentication endpoint containing their credentials (email and password).
- We look up the user record based on the email, and check if the password provided is the correct one for the user. If it’s not, then we send an error response.
- If the password is correct, we use our app.models.Tokens.New() method to generate a token with an expiry time of 24 hours and the scope "authentication".
- We send this authentication token back to the client in a JSON response body.

We need to update `internal/data/tokens.go` file to define a new "authentication" scope, and add some struct tags to customize how the Token struct appears when it is encoded to JSON.

```go
const (
    ScopeActivation     = "activation"
    ScopeAuthentication = "authentication" // Include a new authentication scope.
)

// Add struct tags to control how the struct appears when encoded to JSON.
type Token struct {
    Plaintext string    `json:"token"`
    Hash      []byte    `json:"-"`
    UserID    int64     `json:"-"`
    Expiry    time.Time `json:"expiry"`
    Scope     string    `json:"-"`
}
```

These new struct tags mean that only the Plaintext and Expiry fields will be included when encoding a Token struct — all the other fields will be omitted.
```shell
{
    "token": "X3ASTT2CDAN66BACKSCI4SU7SI",
    "expiry": "2021-01-18T13:00:25.648511827+01:00"
}
```

### Building the endpoint
`POST	/v1/tokens/authentication	createAuthenticationTokenHandler	Generate a new authentication token`

File: cmd/api/tokens.go
```go
func (app *application) createAuthenticationToken(w http.ResponseWriter, r *http.Request) {
	// Parse the email and password from the request body.
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readBody(w, r, &input)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	// Validate the email and password provided by the client.
	v := validator.New()

	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		app.failedValidation(w, r, v.Errors)
		return
	}

	// Lookup the user record based on the email address. If no matching user was
	// found, then we call the app.invalidCredentialsResponse() helper to send a 401
	// Unauthorized response to the client (we will create this helper in a moment).
	user, err := app.repos.User.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrRecordNotFound):
			app.invalidCredentials(w, r)
		default:
			app.serverError(w, r, err)
		}
		return
	}

	// Check if the provided password matches the actual password for the user.
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// If the passwords don't match, then we call the app.invalidCredentialsResponse()
	// helper again and return.
	if !match {
		app.invalidCredentials(w, r)
		return
	}

	// Otherwise, if the password is correct, we generate a new token with a 24-hour
	// expiry time and the scope 'authentication'.
	token, err := app.repos.Token.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Encode the token to JSON and send it in the response along with a 201 Created
	// status code.
	err = app.write(w, http.StatusCreated, envelope{"authentication_token": token}, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}
```

quickly create the `invalidCredentialsResponse()` helper
```go
func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
    message := "invalid authentication credentials"
    app.errorResponse(w, r, http.StatusUnauthorized, message)
}
```

File: cmd/api/routes.go
```go
// Add the route for the POST /v1/tokens/authentication endpoint.
    router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)
```

With all that complete, we should now be able to generate an authentication token.
```shell
$ BODY='{"email": "alice@example.com", "password": "pa55word"}'
$ curl -i -d "$BODY" localhost:4000/v1/tokens/authentication
HTTP/1.1 201 Created
Content-Type: application/json
Date: Fri, 16 Apr 2021 09:03:36 GMT
Content-Length: 125

{
    "authentication_token": {
        "token": "IEYZQUBEMPPAKPOAWTPV6YJ6RM",
        "expiry": "2021-04-17T11:03:36.767078518+02:00"
    }
}
```

Error response:
```shell
$ BODY='{"email": "alice@example.com", "password": "wrong pa55word"}'
$ curl -i -d "$BODY" localhost:4000/v1/tokens/authentication
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Date: Fri, 16 Apr 2021 09:54:01 GMT
Content-Length: 51

{
    "error": "invalid authentication credentials"
}
```

### The Authorization header
Occasionally you might come across other APIs or tutorials where authentication tokens are sent back to the client in an Authorization header, rather than in the response body like we are in this chapter.

You can do that, and in most cases it will probably work fine. But it’s important to be conscious that you’re making a willful violation of the HTTP specifications: Authorization is a request header, not a response header.

## Authenticating Requests
Now that our clients have a way to exchange their credentials for an authentication token, let’s look at how we can use that token to authenticate them, so we know exactly which user a request is coming from.

Once a client has an authentication token we will expect them to include it with all subsequent requests in an Authorization header.
`Authorization: Bearer IEYZQUBEMPPAKPOAWTPV6YJ6RM`

When we receive these requests, we’ll use a new authenticate() middleware method to execute the following logic:

- If the authentication token is not valid, then we will send the client a 401 Unauthorized response and an error message to let them know that their token is malformed or invalid.
- If the authentication token is valid, we will look up the user details and add their details to the request context.
- If no Authorization header was provided at all, then we will add the details for an anonymous user to the request context instead.

### Creating the anonymous user
File: internal/data/users.go
```go
// Declare a new AnonymousUser variable.
var AnonymousUser = &User{}

type User struct {
    ID        int64     `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Password  password  `json:"-"`
    Activated bool      `json:"activated"`
    Version   int       `json:"-"`
}

// Check if a User instance is the AnonymousUser.
func (u *User) IsAnonymous() bool {
    return u == AnonymousUser
}
```

So here we’ve created a new AnonymousUser variable, which holds a pointer to a User struct representing an inactivated user with no ID, name, email or password.

### Reading and writing to the request context
- Every http.Request that our application processes has a context.Context embedded in it, which we can use to store key/value pairs containing arbitrary data during the lifetime of the request. In this case we want to store a User struct containing the current user’s information.

- Any values stored in the request context have the type any. This means that after retrieving a value from the request context you need to assert it back to its original type before using it.

- It’s good practice to use your own custom type for the request context keys. This helps prevent naming collisions between your code and any third-party packages which are also using the request context to store information.

cmd/api/context.go
```go
// Define a custom contextKey type, with the underlying type string.
type contextKey string

// Convert the string "user" to a contextKey type and assign it to the userContextKey
// constant. We'll use this constant as the key for getting and setting user information
// in the request context.
const userContextKey = contextKey("user")

// The contextSetUser() method returns a new copy of the request with the provided
// User struct added to the context. Note that we use our userContextKey constant as the
// key.
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
    ctx := context.WithValue(r.Context(), userContextKey, user)
    return r.WithContext(ctx)
}

// The contextGetUser() retrieves the User struct from the request context. The only
// time that we'll use this helper is when we logically expect there to be User struct
// value in the context, and if it doesn't exist it will firmly be an 'unexpected' error.
// As we discussed earlier in the book, it's OK to panic in those circumstances.
func (app *application) contextGetUser(r *http.Request) *data.User {
    user, ok := r.Context().Value(userContextKey).(*data.User)
    if !ok {
        panic("missing user value in request context")
    }

    return user
}
```

### Creating the authentication middleware
File: cmd/api/middleware.go

```go
func (app *application) authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add the "Vary: Authorization" header to the response. This indicates to any
        // caches that the response may vary based on the value of the Authorization
        // header in the request.
        w.Header().Add("Vary", "Authorization")

        // Retrieve the value of the Authorization header from the request. This will
        // return the empty string "" if there is no such header found.
        authorizationHeader := r.Header.Get("Authorization")

        // If there is no Authorization header found, use the contextSetUser() helper
        // that we just made to add the AnonymousUser to the request context. Then we 
        // call the next handler in the chain and return without executing any of the
        // code below.
        if authorizationHeader == "" {
            r = app.contextSetUser(r, data.AnonymousUser)
            next.ServeHTTP(w, r)
            return
        }

        // Otherwise, we expect the value of the Authorization header to be in the format
        // "Bearer <token>". We try to split this into its constituent parts, and if the
        // header isn't in the expected format we return a 401 Unauthorized response
        // using the invalidAuthenticationTokenResponse() helper (which we will create 
        // in a moment).
        headerParts := strings.Split(authorizationHeader, " ")
        if len(headerParts) != 2 || headerParts[0] != "Bearer" {
            app.invalidAuthenticationTokenResponse(w, r)
            return
        }

        // Extract the actual authentication token from the header parts.
        token := headerParts[1]

        // Validate the token to make sure it is in a sensible format.
        v := validator.New()

        // If the token isn't valid, use the invalidAuthenticationTokenResponse() 
        // helper to send a response, rather than the failedValidationResponse() helper 
        // that we'd normally use.
        if data.ValidateTokenPlaintext(v, token); !v.Valid() {
            app.invalidAuthenticationTokenResponse(w, r)
            return
        }

        // Retrieve the details of the user associated with the authentication token,
        // again calling the invalidAuthenticationTokenResponse() helper if no 
        // matching record was found. IMPORTANT: Notice that we are using 
        // ScopeAuthentication as the first parameter here.
        user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
        if err != nil {
            switch {
            case errors.Is(err, data.ErrRecordNotFound):
                app.invalidAuthenticationTokenResponse(w, r)
            default:
                app.serverErrorResponse(w, r, err)
            }
            return
        }

        // Call the contextSetUser() helper to add the user information to the request
        // context.
        r = app.contextSetUser(r, user)

        // Call the next handler in the chain.
        next.ServeHTTP(w, r)
    })
}
```

- If a valid authentication token is provided in the Authorization header, then a User struct containing the corresponding user details will be stored in the request context.
- If no Authorization header is provided at all, our AnonymousUser struct will be stored in the request context.
- If the Authorization header is provided, but it’s malformed or contains an invalid value, the client will be sent a 401 Unauthorized response using the invalidAuthenticationTokenResponse() helper.

File: cmd/api/errors.go
```go
func (app *application) invalidAuthenticationTokenResponse(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("WWW-Authenticate", "Bearer")

    message := "invalid or missing authentication token"
    app.errorResponse(w, r, http.StatusUnauthorized, message)
}
```

> We’re including a WWW-Authenticate: Bearer header here to help inform or remind the client that we expect them to authenticate using a bearer token.

We need to add the authenticate() middleware to our handler chain. We want to use this middleware on all requests — after our panic recovery and rate limiter middleware, but before our router.

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

    router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
    router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)

    router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)

    // Use the authenticate() middleware on all requests.
    return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
```

### Demonstration
Anonymous user
```shell
$ curl localhost:4000/v1/healthcheck
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
```

Then let’s try the same thing but with a valid authentication token in the Authorization header.
```shell
$ curl -d '{"email": "alice@example.com", "password": "pa55word"}' localhost:4000/v1/tokens/authentication
{
    "authentication_token": {
        "token": "FXCZM44TVLC6ML2NXTOW5OHFUE",
        "expiry": "2021-04-17T12:20:30.02833444+02:00"
    }
}

$ curl -H "Authorization: Bearer FXCZM44TVLC6ML2NXTOW5OHFUE" localhost:4000/v1/healthcheck
{
    "status": "available",
    "system_info": {
        "environment": "development",
        "version": "1.0.0"
    }
}
```




