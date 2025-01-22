package main

import (
	"fmt"
	"net/http"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event of a panic as Go unwinds the stack).

		defer func() {
			// Use the builtin recover function to check if there has been a panic or not.
			if err := recover(); err != nil {
				// This acts as a trigger to make Go's HTTP server
				// automatically close the current connection after a response has been
				// sent.
				w.Header().Set("Connection", "close")

				// The value returned by recover() has the type any, so we use
				// fmt.Errorf() to normalize it into an error.
				// This will log the error using our custom Logger type at the ERROR level
				// and send the client a 500 Internal Server Error response.
				app.serverError(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{w, http.StatusOK}

		defer func() {
			app.logger.Info("debugging info",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
			)
		}()

		next.ServeHTTP(rw, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}
