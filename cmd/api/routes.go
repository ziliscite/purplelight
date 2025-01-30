package main

import (
	"expvar"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFound)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowed)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheck)

	router.HandlerFunc(http.MethodPost, "/v1/anime", app.requirePermission("anime:write", app.createAnime))
	router.HandlerFunc(http.MethodGet, "/v1/anime/:id", app.requirePermission("anime:read", app.showAnime))
	router.HandlerFunc(http.MethodPut, "/v1/anime/:id", app.requirePermission("anime:write", app.updateAnime))
	router.HandlerFunc(http.MethodPatch, "/v1/anime/:id", app.requirePermission("anime:write", app.partiallyUpdateAnime))
	router.HandlerFunc(http.MethodDelete, "/v1/anime/:id", app.requirePermission("anime:write", app.deleteAnime))

	router.HandlerFunc(http.MethodGet, "/v1/anime", app.requirePermission("anime:read", app.listAnime))
	router.HandlerFunc(http.MethodGet, "/v1/tags", app.requirePermission("anime:read", app.listTags))

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUser)
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUser)

	// login, in short
	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationToken)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/activation", app.createActivationToken)

	// Register a new GET /v1/metrics endpoint pointing to the expvar handler.
	router.Handler(http.MethodGet, "/v1/metrics", expvar.Handler())

	// the middleware chain goes -> recoverPanic -> rateLimit -> logging
	// So it works by first calling recoverPanic, then rateLimit, and finally logging
	// which means, if recoverPanic panics, then rateLimit will not be called
	// and if rate limit returns 429, then logging will not be called
	//
	// so I'll reverse the order in the middleware chain.
	// logging -> recoverPanic -> rateLimit
	// so that if recoverPanic panics, then logging will be called
	// and if rate limit returns 429, then logging will also be called
	return app.metrics(app.logging(app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router))))))
}
