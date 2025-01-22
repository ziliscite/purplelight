package main

import (
	"errors"
	"github.com/ziliscite/purplelight/internal/data"
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

func (a animeRequest) nilCheck() error {
	if a.Title == nil {
		return errors.New("title is required")
	}

	if a.Type == nil {
		return errors.New("type is required")
	}

	if a.Status == nil {
		return errors.New("status is required")
	}

	return nil
}

func (a animeRequest) toPost() *data.Anime {
	err := a.nilCheck()
	if err != nil {
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

func (a animeRequest) toPut(anime *data.Anime) error {
	err := a.nilCheck()
	if err != nil {
		return err
	}

	anime.Title = *a.Title
	anime.Type = *a.Type
	anime.Episodes = a.Episodes
	anime.Status = *a.Status
	anime.Season = a.Season
	anime.Year = a.Year
	anime.Duration = a.Duration
	anime.Tags = a.Tags

	return nil
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
