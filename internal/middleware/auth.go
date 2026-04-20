package middleware

import (
	"coffeeshop/internal/config"

	"github.com/gin-gonic/gin"
)

func RequireAuth(cfg *config.Config) gin.HandlerFunc {
	// return func()
}