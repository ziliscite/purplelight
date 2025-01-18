package data

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type Season string

const (
	Spring Season = "Spring"
	Summer Season = "Summer"
	Fall   Season = "Fall"
	Winter Season = "Winter"
)

func (s Season) String() string {
	return string(s)
}

func (s *Season) Set(value string) {
	*s = Season(value)
}

func (s *Season) Scan(value interface{}) error {
	if value == nil {
		return ErrNilValue
	}
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w Season: %T", ErrFailedScan, value)
	}
	s.Set(str)
	return nil
}

func (s Season) Value() (driver.Value, error) {
	return s.String(), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// When Go is decoding a particular type from JSON,
// it looks to see if the type has a UnmarshalJSON() method implemented on it.
// If it has, then Go will call this method to determine how to decode it.
func (s *Season) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch Season(str) {
	case Spring, Summer, Fall, Winter:
		s.Set(str)
		return nil
	default:
		return fmt.Errorf("%w Season: %s", ErrInvalid, s)
	}
}
