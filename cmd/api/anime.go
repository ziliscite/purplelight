package main

import (
	"fmt"
	"net/http"
)

// Add a createAnimeHandler for the "POST /v1/anime" endpoint. For now we simply
// return a plain-text placeholder response.
func (app *application) createAnimeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "create a new movie")
}

// Add a showAnimeHandler for the "GET /v1/anime/:id" endpoint. For now, we retrieve
// the interpolated "id" parameter from the current URL and include it in a placeholder
// response.
func (app *application) showAnimeHandler(w http.ResponseWriter, r *http.Request) {
	// If the parameter couldn't be converted, or is less than 1, we know the ID is invalid
	// so we use the http.NotFound() function to return a 404 Not Found response.
	id, err := app.readIDParam(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Otherwise, interpolate the movie ID in a placeholder response.
	fmt.Fprintf(w, "show the details of movie %d\n", id)
}
