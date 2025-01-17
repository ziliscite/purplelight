# Chapter 4

This chapter will discuss how to read and parse JSON requests from clients.

Key points:
- How to read a request body and decode it to a native Go object using the encoding/json package.  
- How to deal with bad requests from clients and invalid JSON, and return clear, actionable, error messages.  
- How to create a reusable helper package for validating data to ensure it meets your business rules.  
- Different techniques for controlling and customizing how JSON is decoded.

### Decoding

We can use a `json.Decoder` type or using the `json.Unmarshal()` function.

Generally, using json.Decoder is the best choice. It’s more efficient than json.Unmarshal(), requires less code, and offers some helpful settings that you can use to tweak its behavior.  

You can decode a struct like this
```go
var input struct {
    Title   string   `json:"title"`
    Year    int32    `json:"year"`
    Runtime int32    `json:"runtime"`
    Genres  []string `json:"genres"`
}

err := json.NewDecoder(r.Body).Decode(&input)
if err != nil {
    app.errorResponse(w, r, http.StatusBadRequest, err.Error())
    return
}

fmt.Fprintf(w, "%+v\n", input)
```

Points:
- When calling Decode() you must pass a non-nil pointer as the target destination. If you don’t, it will return a json.InvalidUnmarshalError error at runtime.
- If the target decode destination is a struct, the struct fields must be exported. Just like with encoding.
- When decoding a JSON object into a struct, the key/value pairs in the JSON are mapped to the struct fields based on the struct tag names. If there is no matching struct tag, Go will attempt to decode the value into a field that matches the key name (exact matches are preferred, but it will fall back to a case-insensitive match). Any JSON key/value pairs which cannot be successfully mapped to the struct fields will be silently ignored.
- There is no need to close r.Body after it has been read. This will be done automatically by Go’s http.Server, so you don’t have to.


> What happens if we omit a particular key/value pair in our JSON request body?
```shell
$ BODY='{"title":"Moana","runtime":107, "genres":["animation","adventure"]}'
$ curl -d "$BODY" localhost:4000/v1/movies
{Title:Moana Year:0 Runtime:107 Genres:[animation adventure]}
```
When we do this the Year field in our input struct is left with its zero value

This leads to an interesting question: how can you tell the difference between a client not providing a key/value pair, and providing a key/value pair but deliberately setting it to its zero value?

> json.Unmarshal vs json.Decoder  

We can use it like this
```go
var input struct {
    Foo string `json:"foo"`
}

// Use io.ReadAll() to read the entire request body into a []byte slice.
body, err := io.ReadAll(r.Body)
if err != nil {
    app.serverErrorResponse(w, r, err)
    return
}

err = json.Unmarshal(body, &input)
if err != nil {
    app.errorResponse(w, r, http.StatusBadRequest, err.Error())
    return
}

fmt.Fprintf(w, "%+v\n", input)
```

However, this makes it more verbose and less efficient. Look at his benchmark:  
```shell
$ go test -run=^$ -bench=. -benchmem -count=3 -benchtime=5s
goos: linux
goarch: amd64
BenchmarkUnmarshal-8      528088      9543 ns/op     2992 B/op     20 allocs/op
BenchmarkUnmarshal-8      554365     10469 ns/op     2992 B/op     20 allocs/op
BenchmarkUnmarshal-8      537139     10531 ns/op     2992 B/op     20 allocs/op
BenchmarkDecoder-8        811063      8644 ns/op     1664 B/op     21 allocs/op
BenchmarkDecoder-8        672088      8529 ns/op     1664 B/op     21 allocs/op
BenchmarkDecoder-8       1000000      7573 ns/op     1664 B/op     21 allocs/op
```

### Manage Bad Request
Now, what ifs  
- What if the client sends something that isn’t JSON, like XML or some random bytes?
- What happens if the JSON is malformed or contains an error?
- What if the JSON types don’t match the types we are trying to decode into?
- What if the request doesn’t even contain a body?

```shell
# Send some XML as the request body
$ curl -d '<?xml version="1.0" encoding="UTF-8"?><note><to>Alice</to></note>' localhost:4000/v1/movies
{
    "error": "invalid character '\u003c' looking for beginning of value"
}

# Send some malformed JSON (notice the trailing comma)
$ curl -d '{"title": "Moana", }' localhost:4000/v1/movies
{
    "error": "invalid character '}' looking for beginning of object key string"
}

# Send a JSON array instead of an object
$ curl -d '["foo", "bar"]' localhost:4000/v1/movies
{
    "error": "json: cannot unmarshal array into Go value of type struct { Title string 
    \"json:\\\"title\\\"\"; Year int32 \"json:\\\"year\\\"\"; Runtime int32 \"json:\\
    \"runtime\\\"\"; Genres []string \"json:\\\"genres\\\"\" }"
}

# Send a numeric 'title' value (instead of string)
$ curl -d '{"title": 123}' localhost:4000/v1/movies
{
    "error": "json: cannot unmarshal number into Go struct field .title of type string"
}

# Send an empty request body
$ curl -X POST localhost:4000/v1/movies
{
    "error": "EOF"
}
```

When it receives an invalid request that can’t be decoded into our input struct, no further processing takes place, and the error is returned to the client.

> For a private API which won’t be used by members of the public, then this behavior is probably fine and you needn’t do anything else.
> But for a public-facing API, the error messages themselves aren’t ideal. Some are too detailed and expose information about the underlying API implementation. Others aren’t descriptive enough (like "EOF"), and some are just plain confusing and difficult to understand. There isn’t consistency in the formatting or language used either.

To solve it, we're going to `triage the errors`

At this point in this app, the Decode() method could potentially return the following five types of error:

| Error types               | Reason                                                                                     |
|---------------------------|--------------------------------------------------------------------------------------------|
| json.SyntaxError           | There is a syntax problem with the JSON being decoded.                                      |
| io.ErrUnexpectedEOF        | There is a syntax problem with the JSON being decoded.                                      |
| json.UnmarshalTypeError    | A JSON value is not appropriate for the destination Go type.                               |
| json.InvalidUnmarshalError | The decode destination is not valid (usually because it is not a pointer). This is a problem with our application code, not the JSON itself. |
| io.EOF                     | The JSON being decoded is empty.                                                           |


> How do we triage these errors?
```go
// Decode the request body into the target destination. 
err := json.NewDecoder(r.Body).Decode(dst)
if err != nil {
    // If there is an error during decoding, start the triage...
    var syntaxError *json.SyntaxError
    var unmarshalTypeError *json.UnmarshalTypeError
    var invalidUnmarshalError *json.InvalidUnmarshalError

    switch {
    // Use the errors.As() function to check whether the error has the type 
    // *json.SyntaxError. If it does, then return a plain-english error message 
    // which includes the location of the problem.
    case errors.As(err, &syntaxError):
        return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

    // In some circumstances Decode() may also return an io.ErrUnexpectedEOF error
    // for syntax errors in the JSON. So we check for this using errors.Is() and
    // return a generic error message. There is an open issue regarding this at
    // https://github.com/golang/go/issues/25956.
    case errors.Is(err, io.ErrUnexpectedEOF):
        return errors.New("body contains badly-formed JSON")

    // Likewise, catch any *json.UnmarshalTypeError errors. These occur when the
    // JSON value is the wrong type for the target destination. If the error relates
    // to a specific field, then we include that in our error message to make it 
    // easier for the client to debug.
    case errors.As(err, &unmarshalTypeError):
        if unmarshalTypeError.Field != "" {
            return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
        }
        return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

    // An io.EOF error will be returned by Decode() if the request body is empty. We
    // check for this with errors.Is() and return a plain-english error message 
    // instead.
    case errors.Is(err, io.EOF):
        return errors.New("body must not be empty")

    // A json.InvalidUnmarshalError error will be returned if we pass something 
    // that is not a non-nil pointer to Decode(). We catch this and panic, 
    // rather than returning an error to our handler. At the end of this chapter 
    // we'll talk about panicking versus returning errors, and discuss why it's an 
    // appropriate thing to do in this specific situation.
    case errors.As(err, &invalidUnmarshalError):
        panic(err)

    // For anything else, return the error message as-is.
    default:
        return err
    }
}

// If there was no error, then we return nil to indicate success.
return nil
```

The error messages are now simpler, clearer, and consistent in their formatting, and they don’t expose any unnecessary information about our underlying program.

- Why panic?
As you can see in the example above, we panic() if Decode() returns an error on invalid unmarshall error
`in some specific circumstances` — it can be OK to panic.

To our readJSON() helper, if we get a json.InvalidUnmarshalError at runtime it’s because we as the developers have passed an unsupported value to Decode(). 
This is firmly an unexpected error which we shouldn’t see under normal operation, and is something that should be picked up in development and tests long before deployment.

> A panic typically means something went unexpectedly wrong. Mostly we use it to fail fast on errors that shouldn’t occur during normal operation and that we aren’t prepared to handle gracefully.

### Restricting Inputs

We've previously dealt with invalid JSON and other bad request.

But we can still do something about dealing with unknown fields.  
For example:
```go
$ curl -i -d '{"title": "Moana", "rating":"PG"}' localhost:4000/v1/movies
HTTP/1.1 200 OK
Date: Tue, 06 Apr 2021 18:51:50 GMT
Content-Length: 41
Content-Type: text/plain; charset=utf-8

{Title:Moana Year:0 Runtime:0 Genres:[]}
```

Notice how this request works without any problems — there’s no error to inform the client that the rating field is not recognized by our application.  
In our case it would be better if we could alert the client to the issue.

Fortunately, Go’s json.Decoder provides a DisallowUnknownFields() setting that we can use to generate an error when this happens.

> Another problem we have is the fact that json.Decoder is designed to support streams of JSON data. When we call Decode() on our request body, it actually reads the first JSON value only from the body and decodes it. If we made a second call to Decode(), it would read and decode the second value and so on.

But because we call Decode() once — and only once — in our readJSON() helper, anything after the first JSON value in the request body is ignored.

```shell
# Body contains multiple JSON values
$ curl -i -d '{"title": "Moana"}{"title": "Top Gun"}' localhost:4000/v1/movies
HTTP/1.1 200 OK
Date: Tue, 06 Apr 2021 18:53:57 GMT
Content-Length: 41
Content-Type: text/plain; charset=utf-8

{Title:Moana Year:0 Runtime:0 Genres:[]}

# Body contains garbage content after the first JSON value
$ curl -i -d '{"title": "Moana"} :~()' localhost:4000/v1/movies
HTTP/1.1 200 OK
Date: Tue, 06 Apr 2021 18:54:15 GMT
Content-Length: 41
Content-Type: text/plain; charset=utf-8
```

We can handle it through:  
```go
// Use http.MaxBytesReader() to limit the size of the request body to 1MB.
maxBytes := 1_048_576
r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

// Initialize the json.Decoder, and call the DisallowUnknownFields() method on it
// before decoding. This means that if the JSON from the client now includes any
// field which cannot be mapped to the target destination, the decoder will return
// an error instead of just ignoring the field.
dec := json.NewDecoder(r.Body)
dec.DisallowUnknownFields()

// Decode the request body to the destination.
err := dec.Decode(dst)
if err != nil {
	...
	
    // Add a new maxBytesError variable.
    var maxBytesError *http.MaxBytesError

    switch {
	    ...

    // If the JSON contains a field which cannot be mapped to the target destination
    // then Decode() will now return an error message in the format "json: unknown
    // field "<name>"". We check for this, extract the field name from the error,
    // and interpolate it into our custom error message. Note that there's an open
    // issue at https://github.com/golang/go/issues/29035 regarding turning this
    // into a distinct error type in the future.
    case strings.HasPrefix(err.Error(), "json: unknown field "):
        fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
        return fmt.Errorf("body contains unknown key %s", fieldName)

    // Use the errors.As() function to check whether the error has the type 
    // *http.MaxBytesError. If it does, then it means the request body exceeded our 
    // size limit of 1MB and we return a clear error message.
    case errors.As(err, &maxBytesError):
        return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)
		
    ...
}

// Call Decode() again, using a pointer to an empty anonymous struct as the
// destination. If the request body only contained a single JSON value this will
// return an io.EOF error. So if we get anything else, we know that there is
// additional data in the request body and we return our own custom error message.
err = dec.Decode(&struct{}{})
if !errors.Is(err, io.EOF) {
    return errors.New("body must only contain a single JSON value")
}

return nil
```

Now, the previous request will return this error:
```go
$ curl -d '{"title": "Moana", "rating":"PG"}' localhost:4000/v1/movies
{
    "error": "body contains unknown key \"rating\""
}

$ curl -d '{"title": "Moana"}{"title": "Top Gun"}' localhost:4000/v1/movies
{
    "error": "body must only contain a single JSON value"
}

$ curl -d '{"title": "Moana"} :~()' localhost:4000/v1/movies
{
    "error": "body must only contain a single JSON value"
}
```

### Custom JSON Decoding
In the previous chapter, we make it so that runtime information was displayed in the format `<runtime> mins` in our JSON responses.  
Here, we want to make it so that our json reader accepts the format `<runtime> min`.

Right now, its not possible
```go
$ curl -d '{"title": "Moana", "runtime": "107 mins"}' localhost:4000/v1/movies
{
    "error": "body contains incorrect JSON type for \"runtime\""
}
```

To make this work, what we need to do is intercept the decoding process and manually convert the "<runtime> mins" JSON string into an int32 instead.

Similar to marshaller, we need to create a custom unmarshaller. When Go is decoding some JSON, it will check to see if the destination type satisfies the json.Unmarshaler interface.  
The key thing here is knowing about Go’s json.Unmarshaler interface, which looks like this:
```go
type Unmarshaler interface {
    UnmarshalJSON([]byte) error
}
```

The first thing we need to do is update our createMovieHandler so that the input struct uses our custom Runtime type, instead of a regular int32.
```go
var input struct {
    Title   string       `json:"title"`
    Year    int32        `json:"year"`
    Runtime data.Runtime `json:"runtime"` // Make this field a data.Runtime type.
    Genres  []string     `json:"genres"`
}
```

Head to the internal/data/runtime.go file and add a UnmarshalJSON() method to our Runtime type.

```go
// Implement a UnmarshalJSON() method on the Runtime type so that it satisfies the
// json.Unmarshaler interface. IMPORTANT: Because UnmarshalJSON() needs to modify the
// receiver (our Runtime type), we must use a pointer receiver for this to work 
// correctly. Otherwise, we will only be modifying a copy (which is then discarded when 
// this method returns).
func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
    // We expect that the incoming JSON value will be a string in the format 
    // "<runtime> mins", and the first thing we need to do is remove the surrounding 
    // double-quotes from this string. If we can't unquote it, then we return the 
    // ErrInvalidRuntimeFormat error.
    unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
    if err != nil {
        return ErrInvalidRuntimeFormat
    }

    // Split the string to isolate the part containing the number. 
    parts := strings.Split(unquotedJSONValue, " ")

    // Sanity check the parts of the string to make sure it was in the expected format. 
    // If it isn't, we return the ErrInvalidRuntimeFormat error again.
    if len(parts) != 2 || parts[1] != "mins" {
        return ErrInvalidRuntimeFormat
    }

    // Otherwise, parse the string containing the number into an int32. Again, if this
    // fails return the ErrInvalidRuntimeFormat error.
    i, err := strconv.ParseInt(parts[0], 10, 32)
    if err != nil {
        return ErrInvalidRuntimeFormat
    }

    // Convert the int32 to a Runtime type and assign this to the receiver. Note that we
    // use the * operator to deference the receiver (which is a pointer to a Runtime 
    // type) in order to set the underlying value of the pointer.
    *r = Runtime(i)

    return nil
}
```

Now's this the result:
```shell
$ curl -d '{"title": "Moana", "runtime": "107 mins"}' localhost:4000/v1/movies
{Title:Moana Year:0 Runtime:107 Genres:[]}

$ curl -d '{"title": "Moana", "runtime": 107}' localhost:4000/v1/movies
{
        "error": "invalid runtime format"
}

$ curl -d '{"title": "Moana", "runtime": "107 minutes"}' localhost:4000/v1/movies
{
        "error": "invalid runtime format"
}
```

