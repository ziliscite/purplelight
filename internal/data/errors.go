package data

import "errors"

var ErrNilValue = errors.New("value is null")
var ErrFailedScan = errors.New("failed to scan")
var ErrInvalid = errors.New("invalid")
