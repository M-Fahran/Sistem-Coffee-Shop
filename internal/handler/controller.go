package handler

import (
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"
	"net/http"

	"github.com/gin-gonic/gin"
	// "github.com/gin-gonic/gin"
	// "github.com/redis/go-redis/v9/internal/interfaces"
)

// type ProductHandler struct {
// 	productService service.ProductService
// }

type UserHandler struct {
	userService *service.UserService
}

// func NewProductHandler(ps service.ProductService) *ProductHandler {
// 	return &ProductHandler{
// 		productService: ps,
// 	}
// }

// func (h *ProductHandler) GetMenu(c *gin.Context) {
// 	products, err := h.productService.GetAvailableMenu(c.Request.Context())

// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{
// 			"status": "error",
// 			"message": "Gagal mengambil data menu: " + err.Error(),
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":  "success",
// 		"message": "Berhasil mengambil daftar menu",
// 		"data":    products,
// 	})
// }

// func GetMenuHandler(c *gin.Context) {

// 	c.JSON(http.StatusOK, gin.H{
// 		"message": "Ini adalah daftar menu coffee shop",
// 		"data": []string{"Americano", "Latte", "Cappuccino"},
// 	})
// }

// func CreateOrderHandler(c *gin.Context) {
// 	c.JSON(http.StatusCreated, gin.H{
// 		"message": "Pesanan berhasil dibuat",
// 	})
// }

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
