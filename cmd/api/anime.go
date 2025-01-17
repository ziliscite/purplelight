package main

import (
	"github.com/ziliscite/purplelight/internal/data"
	"net/http"
	"time"
)

// Add a createAnimeHandler for the "POST /v1/anime" endpoint. For now we simply
// return a plain-text placeholder response.
func (app *application) createAnimeHandler(w http.ResponseWriter, r *http.Request) {
	// Declare an anonymous struct to hold the information that we expect to be in the
	// HTTP request body (note that the field names and types in the struct are a subset
	// of the struct that we created earlier). This struct will be our *target
	// decode destination*.
	var request struct {
		Title    string         `json:"title"`              // Anime title
		Type     data.AnimeType `json:"type,omitempty"`     // Anime type
		Episodes int32          `json:"episodes,omitempty"` // Number of episodes in the anime
		Status   data.Status    `json:"status,omitempty"`   // Status of the anime
		Season   data.Season    `json:"season,omitempty"`   // Season of the anime
		Year     int32          `json:"year,omitempty"`     // Year the anime was released
		Duration data.Duration  `json:"duration,omitempty"` // Anime duration in minutes
		Tags     []string       `json:"tags,omitempty"`     // Slice of genres for the anime (romance, comedy, etc.)
	}

	err := app.read(w, r, &request)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	err = app.write(w, http.StatusCreated, envelope{"anime": request}, nil)
	if err != nil {
		app.serverError(w, r, err)
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
		Duration:  24,
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
