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
	router.HandlerFunc(http.MethodGet, "/v1/anime", app.listAnime)
	router.HandlerFunc(http.MethodGet, "/v1/anime/:id", app.showAnime)
	router.HandlerFunc(http.MethodPut, "/v1/anime/:id", app.updateAnime)
	router.HandlerFunc(http.MethodPatch, "/v1/anime/:id", app.partiallyUpdateAnime)
	router.HandlerFunc(http.MethodDelete, "/v1/anime/:id", app.deleteAnime)

	return app.recoverPanic(app.logging(router))
}
