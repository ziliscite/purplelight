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







