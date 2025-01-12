package data

import "time"

type Anime struct {
	ID       int64     `json:"id"`                 // Unique integer ID for the anime
	Title    string    `json:"title"`              // Anime title
	Type     AnimeType `json:"type,omitempty"`     // Anime type
	Episodes int32     `json:"episodes,omitempty"` // Number of episodes in the anime
	Status   Status    `json:"status,omitempty"`   // Status of the anime
	Season   Season    `json:"season,omitempty"`   // Season of the anime
	Year     int32     `json:"year,omitempty"`     // Year the anime was released
	Duration int32     `json:"duration,omitempty"` // Anime duration in minutes
	Tags     []string  `json:"tags,omitempty"`     // Slice of genres for the anime (romance, comedy, etc.)

	CreatedAt time.Time `json:"-"`       // Timestamp for when the anime is added to our database
	Version   int32     `json:"version"` // The version number starts at 1 and will be incremented each time the anime information is updated
}
