package data

import (
	"fmt"
	"strconv"
)

type Duration int32

// MarshalJSON Implement a MarshalJSON() method on the Duration type so that it satisfies the
// json.Marshaler interface. This should return the JSON-encoded value for the movie
// duration (in our case, it will return a string in the format "<duration> mins").
func (d Duration) MarshalJSON() ([]byte, error) {
	// Generate a string containing the movie duration in the required format.
	jsonValue := fmt.Sprintf("%d mins", d)

	// Use the strconv.Quote() function on the string to wrap it in double quotes. It
	// needs to be surrounded by double quotes in order to be a valid *JSON string*.
	quotedJSONValue := strconv.Quote(jsonValue)

	// Convert the quoted string value to a byte slice and return it.
	return []byte(quotedJSONValue), nil
}
