package validate

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var v = validator.New()

// Struct validates and returns field->message map.
func Struct(s any) map[string]string {
	if err := v.Struct(s); err == nil {
		return nil
	} else if ve, ok := err.(validator.ValidationErrors); ok {
		out := map[string]string{}
		for _, fe := range ve {
			out[strings.ToLower(fe.Field())] = fmt.Sprintf("failed %s validation", fe.Tag())
		}
		return out
	}
	return map[string]string{"body": "invalid request"}
}
