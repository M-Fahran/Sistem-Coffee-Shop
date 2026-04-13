package handler

import (
	"coffeeshop/internal/service"
	"encoding/json"
	"net/http"

	// "github.com/gin-gonic/gin"
	// "github.com/redis/go-redis/v9/internal/interfaces"
)

type ProductHandler struct {
	productService service.ProductService
}

type UserHandler struct {
	service *service.UserService
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

func NewUserHandler(service *service.UserService) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

func (h *UserHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req service.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "format tidak valid", http.StatusBadRequest)
		return
	}

	token, err := h.service.Auth(r.Context(), req)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message" : "login berhasil",
		"token" : token,
	})
}