package exception

import (
	"errors"
	"log"
	"net/http"

	"coffeeshop/internal/support/response"

	"github.com/gin-gonic/gin"
)

// ErrorHandler processes errors set via c.Error() and serializes them
// into a consistent JSON envelope.
//
//	router := gin.New()
//	router.Use(gin.Logger(), exception.Recovery(), exception.ErrorHandler())
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err

		var ex *Exception
		if errors.As(err, &ex) {
			if ex.Code >= http.StatusInternalServerError {
				log.Printf("[ERROR] %s %s → %v", c.Request.Method, c.Request.URL.Path, ex.Err)
			}
			c.JSON(ex.Code, response.Error(ex.Code, ex.ErrorCode, ex.Message, ex.Details))
			return
		}

		// Unknown error — 500.
		log.Printf("[ERROR] %s %s → unhandled: %v", c.Request.Method, c.Request.URL.Path, err)
		c.JSON(
			http.StatusInternalServerError,
			response.Error(http.StatusInternalServerError, "SYS_500", "internal server error", nil),
		)
	}
}

// Recovery replaces Gin's default recovery with consistent JSON output.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("[PANIC] %s %s → %v", c.Request.Method, c.Request.URL.Path, recovered)
		c.AbortWithStatusJSON(
			http.StatusInternalServerError,
			response.Error(http.StatusInternalServerError, "SYS_500", "internal server error", nil),
		)
	})
}