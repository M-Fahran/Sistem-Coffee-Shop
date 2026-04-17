package exception

import (
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// RegisterJSONTagName configures Gin's validator to use json struct tags
// as field names in validation errors.
//
// Without: {"field": "ProductName"}
// With:    {"field": "product_name"}
//
// Call once at startup:
//
//	exception.RegisterJSONTagName()
func RegisterJSONTagName() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}
}