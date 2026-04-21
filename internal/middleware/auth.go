package middleware

import (
	"coffeeshop/internal/config"
	"coffeeshop/internal/support/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RequireAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "Unauthorized", "Token tidak ada", nil))
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error){
			return []byte(cfg.JWT.Secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "Unauthorized", "Token tidak valid", nil))
			return 
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("user_id", claims["sub"])
			c.Set("role", claims["role"])
		}

		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Error(http.StatusForbidden, "Forbidden", "Akses ditolak", nil))
			return 
		}
		c.Next()
	}
}