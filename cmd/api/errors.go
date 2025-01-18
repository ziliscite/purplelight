package main

import (
	"fmt"
	"net/http"
)

// The logError() method is a generic helper for logging an error message along
// with the current request method and URL as attributes in the log entry.
func (app *application) logError(r *http.Request, err error) {
	app.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
}

// The error() method is a generic helper for sending JSON-formatted error
// messages to the client with a given status code. Note that we're using the any
// type for the message parameter, rather than just a string type, as this gives us
// more flexibility over the values that we can include in the response.
func (app *application) error(w http.ResponseWriter, r *http.Request, status int, message any) {

	// Write the response using the write() helper. If this happens to return an
	// error, then log it and fall back to sending the client an empty response with a
	// 500 Internal Server Error status code.
	err := app.write(w, status, envelope{"error": message}, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(500)
	}
}

// The serverError() method will be used when our application encounters an
// unexpected problem at runtime. It logs the detailed error message, then uses the
// error() helper to send a 500 Internal Server Error status code and JSON
// response (containing a generic error message) to the client.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)

	message := "the server encountered a problem and could not process your request"
	app.error(w, r, http.StatusInternalServerError, message)
}

// The notFound() method will be used to send a 404 Not Found status code and
// JSON response to the client.
func (app *application) notFound(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.error(w, r, http.StatusNotFound, message)
}

// The methodNotAllowed() method will be used to send a 405 Method Not Allowed
// status code and JSON response to the client.
func (app *application) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	app.error(w, r, http.StatusMethodNotAllowed, message)
}

// The badRequest() method will be used to send a 400 Bad Request status code
func (app *application) badRequest(w http.ResponseWriter, r *http.Request, err error) {
	app.error(w, r, http.StatusBadRequest, err.Error())
}

// Note that the errors parameter here has the type map[string]string, which is exactly
// the same as the errors map contained in our Validator type.
func (app *application) failedValidation(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.error(w, r, http.StatusUnprocessableEntity, errors)
}
