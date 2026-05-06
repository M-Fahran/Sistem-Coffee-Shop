package controller

import (
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) Login(c *gin.Context) {
	var req service.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(
			http.StatusBadRequest, "Bad Request", "Format JSON tidak valid", err.Error(),
		))
		return
	}

	UserHandler, token, err := h.userService.Auth(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Error(
			http.StatusUnauthorized, "Unauthorized", err.Error(), nil,
		))
		return
	}

	response.OK(c, "Login Berhasil", gin.H{
		"token": token,
		"user": gin.H{
			"id":       UserHandler.ID,
			"email":    UserHandler.Email,
			"username": UserHandler.Username,
			"role":     UserHandler.Role,
		},
	})
}
