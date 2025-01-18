package data

import (
	"github.com/ziliscite/purplelight/internal/validator"
	"time"
)

type Anime struct {
	ID       int64     `json:"id"`                 // Unique integer ID for the anime
	Title    string    `json:"title"`              // Anime title
	Type     AnimeType `json:"type,omitempty"`     // Anime type
	Episodes *int32    `json:"episodes"`           // Number of episodes in the anime
	Status   Status    `json:"status,omitempty"`   // Status of the anime
	Season   *Season   `json:"season,omitempty"`   // Season of the anime
	Year     *int32    `json:"year"`               // Year the anime was released
	Duration *Duration `json:"duration,omitempty"` // Anime duration in minutes
	Tags     []string  `json:"tags,omitempty"`     // Slice of genres for the anime (romance, comedy, etc.)

	CreatedAt time.Time `json:"-"`       // Timestamp for when the anime is added to our database
	Version   int32     `json:"version"` // The version number starts at 1 and will be incremented each time the anime information is updated
}

func ValidateAnime(v *validator.Validator, a *Anime) {
	// Use the Check() method to execute our validation checks. This will add the
	// provided key and error message to the errors map if the check does not evaluate
	// to true. For example, in the first line here we "check that the title is not
	// equal to the empty string". In the second, we "check that the length of the title
	// is less than or equal to 500 bytes" and so on.
	v.Check(a.Title != "", "title", "must be provided")
	v.Check(len(a.Title) <= 500, "title", "must not be more than 500 bytes long")

	v.Check(a.Status != "", "status", "must be provided")

	// non upcoming
	if a.Status != Upcoming {
		v.Check(a.Year != nil, "year", "non upcoming anime year must be provided")
		v.Check(a.Year != nil && *a.Year <= int32(time.Now().Year()), "year", "non upcoming anime year must not be in the future")

		v.Check(a.Episodes != nil, "episodes", "non upcoming anime episodes must be provided")

		v.Check(a.Type != "", "type", "must be provided")

		v.Check(a.Season != nil && *a.Season != "", "season", "must be provided")
		v.Check(a.Duration != nil && *a.Duration != 0, "duration", "must be provided")
	}

	// upcoming and not nil (can nil)
	if a.Status == Upcoming && a.Year != nil {
		v.Check(*a.Year <= int32(time.Now().Year())+5, "year", "upcoming anime year must not be that far in the future")
	}

	// if nil, should not be checked
	if a.Year != nil {
		v.Check(*a.Year >= 1917, "year", "must be greater than 1917")
	}

	if a.Episodes != nil {
		v.Check(*a.Episodes > 0, "episodes", "must be a positive integer")
	}

	if a.Duration != nil {
		v.Check(*a.Duration > 0, "duration", "must be a positive integer")
	}

	v.Check(a.Tags != nil, "tags", "must be provided")
	v.Check(len(a.Tags) >= 1, "tags", "must contain at least 1 tag")
	v.Check(len(a.Tags) <= 15, "tags", "must not contain more than 15 tags")

	// Note that we're using the Unique helper in the line below to check that all
	// values in the input.Genres slice are unique.
	v.Check(validator.Unique(a.Tags), "tags", "must not contain duplicate values")
}
