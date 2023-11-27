package util

import (
	"github.com/pkg/errors"
)

func JoinErrors(errs ...error) error {
	var errStrings []string
	for _, err := range errs {
		if err != nil {
			errStrings = append(errStrings, err.Error())
		}
	}

	if len(errStrings) > 0 {
		return errors.New(joinStringSlice(errStrings, ";"))
	}
	return nil
}

func joinStringSlice(slice []string, separator string) string {
	if len(slice) == 0 {
		return ""
	} else if len(slice) == 1 {
		return slice[0]
	}

	result := slice[0]
	for _, s := range slice[1:] {
		result += separator + s
	}
	return result
}
