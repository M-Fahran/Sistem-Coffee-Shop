package controller

import (
	"log"
	"net/http"
	"strconv"

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
	orders, err := h.ordersService.GetAllOrders(c.Request.Context())

	if err != nil {
		log.Printf("[ordersController.GetAllOrders] IP: %s | URL: %s | Error: %v\n", c.ClientIP(), c.Request.URL.Path, err)
		response.SendError(c, http.StatusInternalServerError, "INTERNAL ERROR", "Gagal mengambil data order", nil,)
		return
	}

	response.OK(c, "Berhasil mengambil data order", orders)
}

func (h *OrdersController) GetOrderDetail(c *gin.Context) {
	idParam := c.Param("id")
	orderID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		response.SendError(c, http.StatusBadRequest, "BAD REQUEST", "FORMAT ID ORDER TIDAK VALID", nil)
		return
	}

	orders, err := h.ordersService.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		log.Printf("[ordersController.GetOrderDetail] IP: %s | URL: %s | ERROR: %v\n", c.ClientIP(), c.Request.URL.Path, err)
		response.SendError(c, http.StatusInternalServerError, "INTERNAL ERROR", "Gagal mengambil data order berdasarkan id", nil,)
		return
	}

	response.OK(c, "berhasil mengambil data order", orders)
}