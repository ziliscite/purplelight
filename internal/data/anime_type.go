package data

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type AnimeType string

const (
	TV      AnimeType = "TV"
	Movie   AnimeType = "Movie"
	OVA     AnimeType = "OVA"
	ONA     AnimeType = "ONA"
	Special AnimeType = "Special"
)

func (a *AnimeType) String() string {
	return string(*a)
}

func (a *AnimeType) Set(value string) {
	*a = AnimeType(value)
}

func (a *AnimeType) Scan(value interface{}) error {
	if value == nil {
		return ErrNilValue
	}
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w AnimeType: %T", ErrFailedScan, value)
	}
	a.Set(str)
	return nil
}

func (a *AnimeType) Value() (driver.Value, error) {
	return a.String(), nil
}

func (a *AnimeType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch AnimeType(s) {
	case TV, Movie, OVA, ONA, Special:
		a.Set(s)
		return nil
	default:
		return fmt.Errorf("%w AnimeType: %s", ErrInvalid, s)
	}
}
