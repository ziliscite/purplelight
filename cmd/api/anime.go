package main

import (
	"errors"
	"fmt"
	"github.com/ziliscite/purplelight/internal/data"
	"github.com/ziliscite/purplelight/internal/repository"
	"github.com/ziliscite/purplelight/internal/validator"
	"net/http"
)

type animeRequest struct {
	Title    string         `json:"title"`
	Type     data.AnimeType `json:"type,omitempty"`
	Episodes *int32         `json:"episodes,"`
	Status   data.Status    `json:"status,omitempty"`
	Season   *data.Season   `json:"season,"`
	Year     *int32         `json:"year,"`
	Duration *data.Duration `json:"duration,"`
	Tags     []string       `json:"tags,omitempty"`
}

func (a animeRequest) toAnime() *data.Anime {
	return &data.Anime{
		Title:    a.Title,
		Type:     a.Type,
		Episodes: a.Episodes,
		Status:   a.Status,
		Season:   a.Season,
		Year:     a.Year,
		Duration: a.Duration,
		Tags:     a.Tags,
	}
}

func (app *application) createAnime(w http.ResponseWriter, r *http.Request) {
	var request animeRequest

	err := app.read(w, r, &request)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	anime := request.toAnime()

	v := validator.New()
	if data.ValidateAnime(v, anime); !v.Valid() {
		app.failedValidation(w, r, v.Errors)
		return
	}

	err = app.repos.Anime.InsertAnime(anime)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicateEntry):
			app.error(w, r, http.StatusConflict, "anime title already exists")
		case errors.Is(err, repository.ErrDeadlockDetected):
			app.error(w, r, http.StatusConflict, "deadlock detected while trying to insert anime")
		case errors.Is(err, repository.ErrTooManyRows) ||
			errors.Is(err, repository.ErrNotNullViolation) ||
			errors.Is(err, repository.ErrStringDataTruncation) ||
			errors.Is(err, repository.ErrDataTypeMismatch) ||
			errors.Is(err, repository.ErrForeignKeyViolation):
			app.badRequest(w, r, err)
		default:
			app.serverError(w, r, err)
		}
		return
	}

	// When sending a HTTP response, we want to include a Location header to let the
	// client know which URL they can find the newly-created resource at. We make an
	// empty http.Header map and then use the Set() method to add a new Location header,
	// interpolating the system-generated ID for our new movie in the URL.
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/anime/%d", anime.ID))

	// Write a JSON response with a 201 Created status code, the movie data in the
	// response body, and the Location header.
	err = app.write(w, http.StatusCreated, envelope{"anime": anime}, headers)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) showAnime(w http.ResponseWriter, r *http.Request) {
	id, err := app.readID(r)
	if err != nil {
		app.notFound(w, r)
		return
	}

	anime, err := app.repos.Anime.GetAnime(id)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrRecordNotFound):
			app.notFound(w, r)
		default:
			app.serverError(w, r, err)
		}
		return
	}

	err = app.write(w, http.StatusOK, envelope{"anime": anime}, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) updateAnime(w http.ResponseWriter, r *http.Request) {
	id, err := app.readID(r)
	if err != nil {
		app.notFound(w, r)
		return
	}

	anime, err := app.repos.Anime.GetAnime(id)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrRecordNotFound):
			app.notFound(w, r)
		default:
			app.serverError(w, r, err)
		}
		return
	}

	var request animeRequest
	err = app.read(w, r, &request)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	anime.Title = request.Title
	anime.Type = request.Type
	anime.Episodes = request.Episodes
	anime.Status = request.Status
	anime.Season = request.Season
	anime.Year = request.Year
	anime.Duration = request.Duration
	anime.Tags = request.Tags

	v := validator.New()
	if data.ValidateAnime(v, anime); !v.Valid() {
		app.failedValidation(w, r, v.Errors)
		return
	}

	err = app.repos.Anime.UpdateAnime(anime)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicateEntry):
			app.error(w, r, http.StatusConflict, "anime title already exists")
		case errors.Is(err, repository.ErrDeadlockDetected):
			app.error(w, r, http.StatusConflict, "deadlock detected while trying to insert anime")
		case errors.Is(err, repository.ErrTooManyRows) ||
			errors.Is(err, repository.ErrNotNullViolation) ||
			errors.Is(err, repository.ErrStringDataTruncation) ||
			errors.Is(err, repository.ErrDataTypeMismatch) ||
			errors.Is(err, repository.ErrForeignKeyViolation):
			app.badRequest(w, r, err)
		default:
			app.serverError(w, r, err)
		}
		return
	}

	err = app.write(w, http.StatusOK, envelope{"anime": anime}, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the movie ID from the URL.
	id, err := app.readID(r)
	if err != nil {
		app.notFound(w, r)
		return
	}

	// Delete the movie from the database, sending a 404 Not Found response to the
	// client if there isn't a matching record.
	err = app.repos.Anime.DeleteAnime(id)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrRecordNotFound):
			app.notFound(w, r)
		default:
			app.serverError(w, r, err)
		}
		return
	}

	// Return a 200 OK status code along with a success message.
	err = app.write(w, http.StatusOK, envelope{"message": "anime successfully deleted"}, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}
