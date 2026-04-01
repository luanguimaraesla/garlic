package utils

import (
	"database/sql/driver"
	"strings"

	"github.com/luanguimaraesla/garlic/errors"
)

type StringSlice []string

// Scan implements sql.Scanner
func (ss *StringSlice) Scan(src interface{}) error {
	if src == nil {
		*ss = nil
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return errors.New(
			errors.KindSystemError,
			"cannot scan into StringSlice",
			errors.Context(
				errors.Field("source_type", src),
			),
		)
	}
	s = strings.Trim(s, "{}")
	if s == "" {
		*ss = []string{}
	} else {
		*ss = strings.Split(s, ",")
	}
	return nil
}

func (ss StringSlice) Value() (driver.Value, error) {
	if ss == nil {
		return nil, nil
	}
	return "{" + strings.Join(ss, ",") + "}", nil
}
