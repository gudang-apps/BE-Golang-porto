package repository

import "errors"

var ErrNotFound = errors.New("record not found")
var ErrAlreadyReviewed = errors.New("SKU request has already been reviewed")
