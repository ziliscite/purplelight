## Chapter 3.1 

##### JSON Is Just Text
JSON is just text. Sure, it has certain control characters that give the text structure and meaning, but fundamentally, it is just text.

So that means you can write a JSON response from your Go handlers 
in the same way that you would write any other text response: 
using w.Write(), io.WriteString() or one of the fmt.Fprint functions. 
In fact, the only special thing we need to do is set a Content-Type: 
application/json header on the response, so that the client knows 
it’s receiving JSON and can interpret it accordingly.

##### JSON Types Encoding
In this chapter we’ve been encoding a map[string]string type to JSON, which resulted in a JSON object with JSON strings as the values in the key/value pairs. But Go supports encoding many other native types too.

The following table summarizes how different Go types are mapped to JSON data types during encoding:

Go type	⇒	JSON type
bool	⇒	JSON boolean
string	⇒	JSON string
int*, uint*, float*, rune	⇒	JSON number
array, slice	⇒	JSON array
struct, map	⇒	JSON object
nil pointers, interface values, slices, maps, etc.	⇒	JSON null
chan, func, complex*	⇒	Not supported
time.Time	⇒	RFC3339-format JSON string
[]byte	⇒	Base64-encoded JSON string
The last two of these are special cases which deserve a bit more explanation:

Go time.Time values (which are actually a struct behind the scenes) will be encoded as a JSON string in RFC 3339 format like "2020-11-08T06:27:59+01:00", rather than as a JSON object.

A []byte slice will be encoded as a base64-encoded JSON string, rather than as a JSON array. So, for example, a byte slice of []byte{'h','e','l','l','o'} would appear as "aGVsbG8=" in the JSON output. The base64 encoding uses padding and the standard character set.

A few other important things to mention:

Encoding of nested objects is supported. So, for example, if you have a slice of structs in Go that will encode to an array of objects in JSON.

Channels, functions and complex number types cannot be encoded. If you try to do so, you’ll get a json.UnsupportedTypeError error at runtime.

Any pointer values will encode as the value pointed to.

##### JSON Marshall Indent
Actual JSON response data is all just on one line with no whitespace.

```shell
$ curl localhost:4000/v1/healthcheck
{"environment":"development","status":"available","version":"1.0.0"}
```

We can make these easier to read in terminals by using the json.MarshalIndent() function to encode our response data, instead of the regular json.Marshal()

```shell
$ curl -i localhost:4000/v1/healthcheck
{
        "environment": "development",
        "status": "available",
        "version": "1.0.0"
}
```

The following benchmarks help to demonstrate the relative performance of json.Marshal() and json.MarshalIndent() using the code in this gist.

```shell
$ go test -run=^$ -bench=. -benchmem -count=3 -benchtime=5s
goos: linux
goarch: amd64
BenchmarkMarshalIndent-8        2177511     2695 ns/op     1472 B/op     18 allocs/op
BenchmarkMarshalIndent-8        2170448     2677 ns/op     1473 B/op     18 allocs/op
BenchmarkMarshalIndent-8        2150780     2712 ns/op     1476 B/op     18 allocs/op
BenchmarkMarshal-8              3289424     1681 ns/op     1135 B/op     16 allocs/op
BenchmarkMarshal-8              3532242     1641 ns/op     1123 B/op     16 allocs/op
BenchmarkMarshal-8              3619472     1637 ns/op     1119 B/op     16 allocs/op
```

In these benchmarks we can see that json.MarshalIndent() takes 65% longer to run and uses around 30% more memory than json.Marshal()

##### Enveloping Response
```shell
{
    "movie": {
        "id": 123,
        "title": "Casablanca",
        "runtime": 102,
        "genres": [
            "drama",
            "romance",
            "war"
        ],
        "version":1
    }
}
```
Notice how the movie data is nested under the key "movie" here, rather than being the top-level JSON object itself?

A few tangible benefits of this:

1. Including a key name (like "movie") at the top-level of the JSON helps make the response more self-documenting. For any humans who see the response out of context, it is a bit easier to understand what the data relates to.

2. It reduces the risk of errors on the client side, because it’s harder to accidentally process one response thinking that it is something different. To get at the data, a client must explicitly reference it via the "movie" key.

3. If we always envelope the data returned by our API, then we mitigate a security vulnerability in older browsers which can arise if you return a JSON array as a response.

```go
// Define an envelope type.
type envelope map[string]any

// Change the data parameter to have the type envelope instead of any.
func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
    js, err := json.MarshalIndent(data, "", "\t")
    if err != nil {
    return err
    }
    
    js = append(js, '\n')
    
    for key, value := range headers {
    w.Header()[key] = value
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    w.Write(js)
    
    return nil
}
```

##### Go Handles JSON, How?
When Go is encoding a particular type to JSON, it looks to see if the type has a MarshalJSON() method implemented on it. If it has, then Go will call this method to determine how to encode it.

Strictly speaking, when Go is encoding a particular type to JSON it looks to see if the type satisfies the json.Marshaler interface, which looks like this:

```go
type Marshaler interface {
    MarshalJSON() ([]byte, error)
}
```

If the type does satisfy the interface, then Go will call its MarshalJSON() method and use the []byte slice that it returns as the encoded JSON value.

If the type doesn’t have a MarshalJSON() method, then Go will fall back to trying to encode it to JSON based on its own internal set of rules.

So, if we want to customize how something is encoded, all we need to do is implement a MarshalJSON() method on it which returns a custom JSON representation of itself in a []byte slice.

##### Custom marshaler
Let’s change this so that it’s encoded as a string with the format "<runtime> mins" instead
```json
{
    "id": 123,
    "title": "Casablanca",
    "runtime": "102 mins",      ← This is now a string
    "genres": [
        "drama",
        "romance",
        "war"
    ],
    "version":1
}
```

Here's how
```go
// Declare a custom Runtime type, which has the underlying type int32 (the same as our
// Movie struct field).
type Runtime int32

// Implement a MarshalJSON() method on the Runtime type so that it satisfies the 
// json.Marshaler interface. This should return the JSON-encoded value for the movie 
// runtime (in our case, it will return a string in the format "<runtime> mins").
func (r Runtime) MarshalJSON() ([]byte, error) {
    // Generate a string containing the movie runtime in the required format.
    jsonValue := fmt.Sprintf("%d mins", r)

    // Use the strconv.Quote() function on the string to wrap it in double quotes. It 
    // needs to be surrounded by double quotes in order to be a valid *JSON string*.
    quotedJSONValue := strconv.Quote(jsonValue)

    // Convert the quoted string value to a byte slice and return it.
    return []byte(quotedJSONValue), nil
}
```

There are two things to emphasize here:

1. If your MarshalJSON() method returns a JSON string value, like ours does, then you must wrap the string in double quotes before returning it. Otherwise it won’t be interpreted as a JSON string and you’ll receive a runtime error similar to this:

```
json: error calling MarshalJSON for type data.Runtime: invalid character 'm' after top-level value
```

2. We’re deliberately using a value receiver for our MarshalJSON() method rather than a pointer receiver like func (r *Runtime) MarshalJSON(). This gives us more flexibility because it means that our custom JSON encoding will work on both Runtime values and pointers to Runtime values. As Effective Go mentions:

> The rule about pointers vs. values for receivers is that value methods can be invoked on pointers and values, but pointer methods can only be invoked on pointers.

Hint: The difference between pointer and value receivers: [this](https://medium.com/globant/go-method-receiver-pointer-vs-value-ffc5ab7acdb) blog post provides a good summary.

##### Error messages
we’re still sending them a plain-text error message from the http.Error() and http.NotFound() functions.

```go
// The logError() method is a generic helper for logging an error message along
// with the current request method and URL as attributes in the log entry.
func (app *application) logError(r *http.Request, err error) {
    var (
        method = r.Method
        uri    = r.URL.RequestURI()
    )
    
    app.logger.Error(err.Error(), "method", method, "uri", uri)
}

// The errorResponse() method is a generic helper for sending JSON-formatted error
// messages to the client with a given status code. Note that we're using the any
// type for the message parameter, rather than just a string type, as this gives us
// more flexibility over the values that we can include in the response.
func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
    env := envelope{"error": message}

    // Write the response using the writeJSON() helper. If this happens to return an
    // error then log it, and fall back to sending the client an empty response with a
    // 500 Internal Server Error status code.
    err := app.writeJSON(w, status, env, nil)
    if err != nil {
        app.logError(r, err)
        w.WriteHeader(500)
    }
}

// custom error response for each method
```
Any error messages that our own API handlers send will now be well-formed JSON responses.

##### Routing errors
Error messages that httprouter automatically sends when it can’t find a matching route? 
By default, these will still be the same plain-text (non-JSON) responses that we saw earlier in the book.

Fortunately, httprouter allows us to set our own custom error handlers when we initialize the router. 
These custom handlers must satisfy the http.Handler interface, 
so we can re-use the notFoundResponse() and methodNotAllowedResponse() helpers.

```go
func (app *application) routes() http.Handler {
    router := httprouter.New()

    // Convert the notFoundResponse() helper to a http.Handler using the 
    // http.HandlerFunc() adapter, and then set it as the custom error handler for 404
    // Not Found responses.
    router.NotFound = http.HandlerFunc(app.notFoundResponse)

    // Likewise, convert the methodNotAllowedResponse() helper to a http.Handler and set
    // it as the custom error handler for 405 Method Not Allowed responses.
    router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

    router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)
    router.HandlerFunc(http.MethodPost, "/v1/movies", app.createMovieHandler)
    router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler)

    return router
}
```

http/net could never, lmao

##### Panic recovery
At the moment any panics in our API handlers will be recovered automatically by Go’s http.Server. 
This will unwind the stack for the affected goroutine (calling any deferred functions along the way), close the underlying HTTP connection, 
and log an error message and stack trace.

This behavior is OK, but it would be better for the client if we could also send a 500 Internal Server Error 
response to explain that something has gone wrong — rather than just closing the HTTP connection with no context.

```go
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event of a panic as Go unwinds the stack).

		defer func() {
			// Use the builtin recover function to check if there has been a panic or not.
			if err := recover(); err != nil {
				// If there was a panic, set a "Connection: close" header on the
				// response. This acts as a trigger to make Go's HTTP server
				// automatically close the current connection after a response has been
				// sent.
				w.Header().Set("Connection", "close")

				// The value returned by recover() has the type any, so we use
				// fmt.Errorf() to normalize it into an error and call our
				// serverError() helper. In turn, this will log the error using
				// our custom Logger type at the ERROR level and send the client a 500
				// Internal Server Error response.
				app.serverError(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
```

##### System generated error response
Go’s http.Server may still automatically generate and send plain-text HTTP responses. These scenarios include when:
- The HTTP request specifies an unsupported HTTP protocol version.
- The HTTP request contains a missing or invalid Host header, or multiple Host headers.
- The HTTP request contains a empty Content-Length header.
- The HTTP request contains an unsupported Transfer-Encoding header.
- The size of the HTTP request headers exceeds the server’s MaxHeaderBytes setting.
- The client makes a HTTP request to a HTTPS server.  

> Unfortunately, these responses are hard-coded into the Go standard library, and there’s nothing we can do to customize them to use JSON instead.

##### Panic recovery in other goroutines
It’s really important to realize that our middleware will only recover panics that happen in the same goroutine that executed the recoverPanic() middleware.

If, for example, you have a handler which spins up another goroutine (e.g. to do some background processing), then any panics that happen in the background goroutine will not be recovered — not by the recoverPanic() middleware… and not by the panic recovery built into http.Server. These panics will cause your application to exit and bring down the server.

So, if you are spinning up additional goroutines from within your handlers and there is any chance of a panic, you must make sure that you recover any panics from within those goroutines too.

We’ll look at this topic in more detail later in the book, and demonstrate how to deal with it when we use a background goroutine to send welcome emails to our API users.

