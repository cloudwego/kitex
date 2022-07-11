package xdsresource

import (
	"fmt"
	"strings"
)

func processUnmarshalErrors(errs []error, errMap map[string]error) error {
	var b strings.Builder
	for _, err := range errs {
		b.WriteString(err.Error())
		b.WriteString("\n")
	}

	if len(errMap) > 0 {
		b.WriteString("Error per resource:\n")
		for name, err := range errMap {
			b.WriteString(fmt.Sprintf("resource %s: %s;\n", name, err.Error()))
		}
	}

	return fmt.Errorf(b.String())
}

func combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	var b strings.Builder
	for _, err := range errs {
		b.WriteString(err.Error())
		b.WriteString("\n")
	}
	return fmt.Errorf(b.String())
}
