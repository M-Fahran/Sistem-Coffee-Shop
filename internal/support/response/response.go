package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Success   bool   `json:"success"`
	Code      int    `json:"code"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Errors    any    `json:"errors,omitempty"`
}

// PageMeta carries pagination parameters.
type PageMeta struct {
	Page    int
	PerPage int
	Total   int
}

func Success(code int, message string, data any) Envelope {
	return Envelope{
		Success: true,
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Error returns an error envelope.
func Error(code int, errCode, message string, details any) Envelope {
	return Envelope{
		Success:   false,
		Code:      code,
		ErrorCode: errCode,
		Message:   message,
		Errors:    details,
	}
}

// OK sends a 200 JSON response.
func OK(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Success(http.StatusOK, message, data))
}

// Created sends a 201 JSON response.
func Created(c *gin.Context, message string, data any) {
	c.JSON(http.StatusCreated, Success(http.StatusCreated, message, data))
}

// NoContent sends a 204 with no body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Paginated sends a 200 JSON response with pagination metadata.
func Paginated(c *gin.Context, message string, items any, meta PageMeta) {
	c.JSON(http.StatusOK, Success(http.StatusOK, message, gin.H{
		"items": items,
		"meta": gin.H{
			"page":      meta.Page,
			"per_page":  meta.PerPage,
			"total":     meta.Total,
			"last_page": lastPage(meta.Total, meta.PerPage),
		},
	}))
}

func lastPage(total, perPage int) int {
	if perPage <= 0 {
		return 1
	}
	lp := total / perPage
	if total%perPage != 0 {
		lp++
	}
	return lp
}