package controller

import (
	"coffeeshop/internal/request"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userHandlerService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userHandlerService: userService}
}

func (h *UserHandler) Login(c *gin.Context) {
	var req request.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.SendError(c, http.StatusBadRequest, "Bad Request", "Format JSON tidak valid", err.Error())
		return
	}

	UserHandler, token, err := h.userHandlerService.Auth(c.Request.Context(), req)
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
