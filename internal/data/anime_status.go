package data

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type Status string

const (
	Ongoing  Status = "Ongoing"
	Finished Status = "Finished"
	Upcoming Status = "Upcoming"
)

func (s *Status) String() string {
	return string(*s)
}

func (s *Status) Set(value string) {
	*s = Status(value)
}

func (s *Status) Scan(value interface{}) error {
	if value == nil {
		return ErrNilValue
	}
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w Status: %T", ErrFailedScan, value)
	}
	s.Set(str)
	return nil
}

func (s *Status) Value() (driver.Value, error) {
	return s.String(), nil
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch Status(str) {
	case Ongoing, Finished, Upcoming:
		s.Set(str)
		return nil
	default:
		return fmt.Errorf("%w Status: %s", ErrInvalid, s)
	}
}
