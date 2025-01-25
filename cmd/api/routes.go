package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFound)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowed)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheck)

	router.HandlerFunc(http.MethodPost, "/v1/anime", app.createAnime)
	router.HandlerFunc(http.MethodGet, "/v1/anime/:id", app.showAnime)
	router.HandlerFunc(http.MethodPut, "/v1/anime/:id", app.updateAnime)
	router.HandlerFunc(http.MethodPatch, "/v1/anime/:id", app.partiallyUpdateAnime)
	router.HandlerFunc(http.MethodDelete, "/v1/anime/:id", app.deleteAnime)

	router.HandlerFunc(http.MethodGet, "/v1/tags", app.listTags)
	router.HandlerFunc(http.MethodGet, "/v1/anime", app.listAnime)

	// the middleware chain goes -> recoverPanic -> rateLimit -> logging
	// So it works by first calling recoverPanic, then rateLimit, and finally logging
	// which means, if recoverPanic panics, then rateLimit will not be called
	// and if rate limit returns 429, then logging will not be called
	//
	// so I'll reverse the order in the middleware chain.
	// logging -> recoverPanic -> rateLimit
	// so that if recoverPanic panics, then logging will be called
	// and if rate limit returns 429, then logging will also be called
	return app.logging(app.recoverPanic(app.rateLimit(router)))
}
