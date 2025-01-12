package main

import (
	"encoding/json"
	"github.com/ziliscite/purplelight/internal/data"
	"net/http"
	"time"
)

// Add a createAnimeHandler for the "POST /v1/anime" endpoint. For now we simply
// return a plain-text placeholder response.
func (app *application) createAnimeHandler(w http.ResponseWriter, r *http.Request) {
	var anime data.Anime
	if err := json.NewDecoder(r.Body).Decode(&anime); err != nil {
		err = app.write(w, http.StatusBadRequest, envelope{"Error": "Invalid request body"}, nil)
		if err != nil {
			app.logger.Error(err.Error())
			http.Error(w, "The server encountered a problem and could not process your request", http.StatusInternalServerError)
			return
		}
		return
	}

	err := app.write(w, http.StatusOK, envelope{"anime": anime}, nil)
	if err != nil {
		app.logger.Error(err.Error())
		http.Error(w, "The server encountered a problem and could not process your request", http.StatusInternalServerError)
	}
}

// Add a showAnimeHandler for the "GET /v1/anime/:id" endpoint. For now, we retrieve
// the interpolated "id" parameter from the current URL and include it in a placeholder
// response.
func (app *application) showAnimeHandler(w http.ResponseWriter, r *http.Request) {
	// If the parameter couldn't be converted, or is less than 1, we know the ID is invalid
	// so we use the http.NotFound() function to return a 404 Not Found response.
	id, err := app.readID(r)
	if err != nil {
		// Use the new notFoundResponse() helper.
		app.notFound(w, r)
		return
	}

	// Otherwise, interpolate the anime ID in a placeholder response.
	anime := data.Anime{
		ID:        id,
		Title:     "Fullmetal Alchemist: Brotherhood",
		Type:      data.TV,
		Episodes:  64,
		Status:    data.Finished,
		Season:    data.Spring,
		Year:      2009,
		Tags:      []string{"Action", "Adventure", "Fantasy"},
		CreatedAt: time.Now(),
		Version:   1,
	}

	// Create an envelope{"movie": movie} instance and pass it to writeJSON(), instead
	// of passing the plain movie struct.
	err = app.write(w, http.StatusOK, envelope{"anime": anime}, nil)
	if err != nil {
		// Use the new serverErrorResponse() helper.
		app.serverError(w, r, err)
	}
}
