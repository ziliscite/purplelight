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
