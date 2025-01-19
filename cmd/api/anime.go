package main

import (
	"errors"
	"github.com/ziliscite/purplelight/internal/data"
	"github.com/ziliscite/purplelight/internal/repository"
	"github.com/ziliscite/purplelight/internal/validator"
	"net/http"
)

func (app *application) createAnime(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Title    string         `json:"title"`
		Type     data.AnimeType `json:"type,omitempty"`
		Episodes *int32         `json:"episodes,"`
		Status   data.Status    `json:"status,omitempty"`
		Season   *data.Season   `json:"season,"`
		Year     *int32         `json:"year,"`
		Duration *data.Duration `json:"duration,"`
		Tags     []string       `json:"tags,omitempty"`
	}

	err := app.read(w, r, &request)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	v := validator.New()

	anime := &data.Anime{
		Title:    request.Title,
		Type:     request.Type,
		Episodes: request.Episodes,
		Status:   request.Status,
		Season:   request.Season,
		Year:     request.Year,
		Duration: request.Duration,
		Tags:     request.Tags,
	}

	if data.ValidateAnime(v, anime); !v.Valid() {
		app.failedValidation(w, r, v.Errors)
		return
	}

	err = app.repos.Anime.InsertAnime(anime)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicateEntry):
			app.error(w, r, http.StatusConflict, map[string]string{"title": "anime title already exists"})
		case !errors.Is(err, repository.ErrInternalDatabase), !errors.Is(err, repository.ErrTransaction), !errors.Is(err, repository.ErrDatabaseUnknown), !errors.Is(err, repository.ErrQueryPrepare), !errors.Is(err, repository.ErrFailedCloseStmt):
			app.badRequest(w, r, err)
		default:
			app.serverError(w, r, err)
		}
		return
	}

	err = app.write(w, http.StatusCreated, envelope{"anime": anime}, nil)
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
