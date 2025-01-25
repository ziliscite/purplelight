package data

import (
	"fmt"
	"strings"
)

var animeTypeMap = map[string]AnimeType{
	"tv":      TV,
	"movie":   Movie,
	"ova":     OVA,
	"ona":     ONA,
	"special": Special,
}

func TypeToEnum(val string) (string, error) {
	key := strings.ToLower(val)
	if at, ok := animeTypeMap[key]; ok {
		return string(at), nil
	}
	return "", fmt.Errorf("%w AnimeType: %s", ErrInvalid, val)
}

var statusMap = map[string]Status{
	"ongoing":  Ongoing,
	"finished": Finished,
	"upcoming": Upcoming,
}

func StatusToEnum(val string) (string, error) {
	key := strings.ToLower(val)
	if st, ok := statusMap[key]; ok {
		return string(st), nil
	}
	return "", fmt.Errorf("%w Status: %s", ErrInvalid, val)
}

var seasonMap = map[string]Season{
	"spring": Spring,
	"summer": Summer,
	"fall":   Fall,
	"winter": Winter,
}

func SeasonToEnum(val string) (string, error) {
	key := strings.ToLower(val)
	if se, ok := seasonMap[key]; ok {
		return string(se), nil
	}
	return "", fmt.Errorf("%w Season: %s", ErrInvalid, val)
}
