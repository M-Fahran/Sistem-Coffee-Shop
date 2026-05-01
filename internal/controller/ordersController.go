package controller

import (
	"log"
	"net/http"

	// "coffeeshop/internal/service"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"

	"github.com/gin-gonic/gin"
)

type OrdersController struct {
	ordersService *service.OrdersService
}

func NewOrdersController(ordersService *service.OrdersService) *OrdersController {
	return &OrdersController{ordersService: ordersService}
}

func (h *OrdersController) GetAllOrders(c *gin.Context) {
	products, err := h.ordersService.GetAllOrders(c.Request.Context())

	if err != nil {
		log.Printf("[ordersController.GetAllOrders] IP: %s | URL: %s | Error: %v\n", c.ClientIP(), c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, response.Error(
			http.StatusInternalServerError,
			"INTERNAL ERROR",
			"Gagal mengambil data produk",
			nil,
		))
		return
	}

	response.OK(c, "Berhasil mengambil produk", products)
}