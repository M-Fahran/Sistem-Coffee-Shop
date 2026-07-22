package table

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/support/exception"
)

// BindIDParam parses :id from URL path.
func BindIDParam(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, exception.BadRequest("TABLE_400", "invalid table id")
	}
	return id, nil
}

// BindQRTokenParam parses :token from URL path.
// QR tokens are exactly 64 hex characters (32 bytes).
func BindQRTokenParam(c *gin.Context) (string, error) {
	token := strings.TrimSpace(c.Param("token"))
	if len(token) != 64 || !isHex(token) {
		return "", exception.NotFound("TABLE_404", "invalid QR token")
	}
	return token, nil
}

func isHex(s string) bool {
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}