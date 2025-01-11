package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func (app *application) routes() http.Handler {
	// Initialize a new `httprouter` router instance.
	router := httprouter.New()

	// Register the relevant methods, URL patterns, and handler functions for our
	// endpoints using the HandlerFunc() method. Note that http.MethodGet and
	// http.MethodPost are constants which equate to the strings "GET" and "POST"
	// respectively.
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)
	router.HandlerFunc(http.MethodPost, "/v1/anime", app.createAnimeHandler)
	router.HandlerFunc(http.MethodGet, "/v1/anime/:id", app.showAnimeHandler)

	// Return the `httprouter` instance.
	return router
}
