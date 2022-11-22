package util

import (
	"strings"
)

type ErrorSet []error

func NewErrorSet() ErrorSet {
	return []error{}
}

func (es ErrorSet) Error() string {
	var sb = strings.Builder{}
	for _, err := range es {
		sb.WriteString(err.Error())
		sb.WriteByte('\n')
	}

	return sb.String()
}

func (es ErrorSet) Err() error {
	if len(es) > 0 {
		return es
	}

	return nil
}
