package main

import (
	"github.com/ziliscite/purplelight/internal/data"
	"github.com/ziliscite/purplelight/internal/validator"
	"net/url"
)

type animeRequest struct {
	Title    *string         `json:"title"`
	Type     *data.AnimeType `json:"type,omitempty"`
	Episodes *int32          `json:"episodes,"`
	Status   *data.Status    `json:"status,omitempty"`
	Season   *data.Season    `json:"season,"`
	Year     *int32          `json:"year,"`
	Duration *data.Duration  `json:"duration,"`
	Tags     []string        `json:"tags,omitempty"`
}

func (a animeRequest) nilCheck(v *validator.Validator) bool {
	if a.Title == nil {
		v.AddError("title", "title should not be nil")
	}

	if a.Type == nil {
		v.AddError("type", "type should not be nil")
	}

	if a.Status == nil {
		v.AddError("status", "status should not be nil")
	}

	return v.Valid()
}

func (a animeRequest) toPost(v *validator.Validator) *data.Anime {
	ok := a.nilCheck(v)
	if !ok {
		return nil
	}

	return &data.Anime{
		Title:    *a.Title,
		Type:     *a.Type,
		Episodes: a.Episodes,
		Status:   *a.Status,
		Season:   a.Season,
		Year:     a.Year,
		Duration: a.Duration,
		Tags:     a.Tags,
	}
}

func (a animeRequest) toPut(anime *data.Anime, v *validator.Validator) {
	ok := a.nilCheck(v)
	if !ok {
		return
	}

	anime.Title = *a.Title
	anime.Type = *a.Type
	anime.Episodes = a.Episodes
	anime.Status = *a.Status
	anime.Season = a.Season
	anime.Year = a.Year
	anime.Duration = a.Duration
	anime.Tags = a.Tags
}

func (a animeRequest) toPatch(anime *data.Anime) {
	// If the input.Title value is nil then we know that no corresponding "title" key/
	// value pair was provided in the JSON request body. So we move on and leave the
	// anime record unchanged. Otherwise, we update the anime record with the new title
	// value. Importantly, because input.Title is a now a pointer to a string, we need
	// to dereference the pointer using the * operator to get the underlying value
	// before assigning it to our anime record.

	if a.Title != nil {
		anime.Title = *a.Title
	}

	if a.Type != nil {
		anime.Type = *a.Type
	}

	if a.Episodes != nil {
		anime.Episodes = a.Episodes
	}

	if a.Status != nil {
		anime.Status = *a.Status
	}

	if a.Season != nil {
		anime.Season = a.Season
	}

	if a.Year != nil {
		anime.Year = a.Year
	}

	if a.Duration != nil {
		anime.Duration = a.Duration
	}

	if a.Tags != nil {
		anime.Tags = a.Tags
	}
}

type animeQuery struct {
	Title     string
	Status    string
	Season    string
	AnimeType string
	Tags      []string
	data.Filters
}

func (aq *animeQuery) readQuery(qs url.Values, app *application, v *validator.Validator) {
	// Use our helpers to extract the title and genres query string values, falling back
	// to defaults of an empty string and an empty slice respectively if they are not
	// provided by the client.
	aq.Title = app.readString(qs, "title", "")
	aq.Tags = app.readCSV(qs, "tags", []string{})

	// Extract the status, season, and type query string values, falling back to the
	// zero value for each type if they are not provided by the client.
	aq.Status = app.readIota(qs, "status", "", v, data.StatusToEnum)

	aq.Season = app.readIota(qs, "season", "", v, data.SeasonToEnum)

	aq.AnimeType = app.readIota(qs, "anime_type", "", v, data.TypeToEnum)

	// Get the page and page_size query string values as integers. Notice that we set
	// the default page value to 1 and default page_size to 20, and that we pass the
	// validator instance as the final argument here.
	aq.Filters.Page = app.readInt(qs, "page", 1, v)
	aq.Filters.PageSize = app.readInt(qs, "page_size", 20, v)

	// Extract the sort query string value, falling back to "id" if it is not provided
	// by the client (which will imply a ascending sort on movie ID).
	aq.Filters.Sort = app.readString(qs, "sort", "id")

	// Add the supported sort values for this endpoint to the sort safelist.
	aq.Filters.SortSafeList = []string{"id", "title", "year", "episodes", "-id", "-title", "-year", "-episodes"}
}
