package api

import (
	"net/http"
	"coffeeshop/internal/service"
	"github.com/gin-gonic/gin"
)

type ProductHandler struct {
	productService service.ProductService
}

func NewProductHandler(ps service.ProductService) *ProductHandler {
	return &ProductHandler{
		productService: ps,
	}
}

func (h *ProductHandler) GetMenu(c *gin.Context) {
	products, err := h.productService.GetAvailableMenu(c.Request.Context())
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"message": "Gagal mengambil data menu: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Berhasil mengambil daftar menu",
		"data":    products,
	})
}

func GetMenuHandler(c *gin.Context) {
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Ini adalah daftar menu coffee shop",
		"data": []string{"Americano", "Latte", "Cappuccino"},
	})
}

func CreateOrderHandler(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"message": "Pesanan berhasil dibuat",
	})
}