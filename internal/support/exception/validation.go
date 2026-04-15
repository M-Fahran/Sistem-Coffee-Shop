package exception

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// FieldError represents a single field validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validation extracts field-level errors from Gin's binding and returns
// a 422 Exception.
//
//	if err := c.ShouldBindJSON(&req); err != nil {
//	    _ = c.Error(exception.Validation(err))
//	    return
//	}
func Validation(err error) *Exception {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make([]FieldError, 0, len(ve))
		for _, fe := range ve {
			fields = append(fields, FieldError{
				Field:   fe.Field(),
				Message: validationMessage(fe),
			})
		}
		return &Exception{
			Code:      http.StatusUnprocessableEntity,
			ErrorCode: "VALIDATION_422",
			Message:   "validation failed",
			Details:   fields,
			Err:       err,
		}
	}

	return New(http.StatusBadRequest, "REQUEST_400", "invalid request body")
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		return fmt.Sprintf("minimum %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("maximum %s characters", fe.Param())
	case "gte":
		return fmt.Sprintf("must be >= %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be <= %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	case "numeric":
		return "must be numeric"
	case "alphanum":
		return "must be alphanumeric"
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	case "uuid":
		return "must be a valid UUID"
	default:
		return fmt.Sprintf("failed on '%s' validation", fe.Tag())
	}
}