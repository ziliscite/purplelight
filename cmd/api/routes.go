package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func (app *application) routes() http.Handler {
	// Initialize a new `httprouter` router instance.
	router := httprouter.New()

	// Convert the notFound() helper to a http.Handler using the
	// http.HandlerFunc() adapter, and then set it as the custom error handler for 404
	// Not Found responses.
	router.NotFound = http.HandlerFunc(app.notFound)

	// Likewise, convert the methodNotAllowed() helper to a http.Handler and set
	// it as the custom error handler for 405 Method Not Allowed responses.
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowed)

	// Register the relevant methods, URL patterns, and handler functions for our
	// endpoints using the HandlerFunc() method. Note that http.MethodGet and
	// http.MethodPost are constants which equate to the strings "GET" and "POST"
	// respectively.
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)
	router.HandlerFunc(http.MethodPost, "/v1/anime", app.createAnimeHandler)
	router.HandlerFunc(http.MethodGet, "/v1/anime/:id", app.showAnimeHandler)

	// Return the `httprouter` instance.
	// Wrap the router with the panic recovery middleware.
	return app.recoverPanic(router)
}
