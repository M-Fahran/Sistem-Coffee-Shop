package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/config"
	"coffeeshop/internal/middleware"
	"coffeeshop/internal/request"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/exception"
	"coffeeshop/internal/support/response"
)

// idempotencyHeader — klien mengirim UUID yang sama saat mencoba ulang
// request yang gagal karena jaringan.
const idempotencyHeader = "Idempotency-Key"

// maxIdempotencyKeyLength menjaga agar key tidak dipakai menitipkan data
// besar ke Redis.
const maxIdempotencyKeyLength = 128

type OrdersController struct {
	svc *service.OrdersService
}

func NewOrdersController(svc *service.OrdersService) *OrdersController {
	return &OrdersController{svc: svc}
}

// ============================================================================
// Pelanggan
// ============================================================================

// CreateCustomerOrder POST /api/v1/customer/orders
//
// table_id TIDAK dibaca dari body. Diambil dari session token hasil scan QR,
// sehingga pelanggan tidak bisa memesan atas nama meja lain.
func (h *OrdersController) CreateCustomerOrder(c *gin.Context) {
	tableID, ok := middleware.TableID(c)
	if !ok {
		_ = c.Error(exception.Unauthorized("SESSION_401", "table session required"))
		return
	}

	req, err := request.BindCreateOrderRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	idemKey, err := idempotencyKey(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	in := req.ToServiceInput(nil, config.OrderSourceQR, nil)
	in.UserAgent = optionalString(c.Request.UserAgent())
	in.IPAddress = optionalString(c.ClientIP())

	order, err := h.svc.CreateCustomerOrder(c.Request.Context(), tableID, idemKey, in)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response.Created(c, "order created successfully", order)
}

// GetCustomerOrder GET /api/v1/customer/orders/:id
//
// Hanya bisa membaca pesanan milik mejanya sendiri.
func (h *OrdersController) GetCustomerOrder(c *gin.Context) {
	tableID, ok := middleware.TableID(c)
	if !ok {
		_ = c.Error(exception.Unauthorized("SESSION_401", "table session required"))
		return
	}

	orderID, err := orderIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	order, err := h.svc.GetTableOrder(c.Request.Context(), orderID, tableID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response.OK(c, "order retrieved successfully", order)
}

// ============================================================================
// Kasir
// ============================================================================

// CreateCashierOrder POST /api/v1/orders
func (h *OrdersController) CreateCashierOrder(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		_ = c.Error(exception.Unauthorized("AUTH_401", "authentication required"))
		return
	}

	req, err := request.BindCashierOrderRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	idemKey, err := idempotencyKey(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	in := req.ToServiceInput(req.TableID, config.OrderSourceCashier, &userID)
	in.UserAgent = optionalString(c.Request.UserAgent())
	in.IPAddress = optionalString(c.ClientIP())

	order, err := h.svc.CreateCashierOrder(c.Request.Context(), userID, req.TableID, idemKey, in)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response.Created(c, "order created successfully", order)
}

// GetAllOrders GET /api/v1/orders
func (h *OrdersController) GetAllOrders(c *gin.Context) {
	orders, err := h.svc.GetAllOrders(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, "orders retrieved successfully", orders)
}

// GetOrderDetail GET /api/v1/orders/:id
func (h *OrdersController) GetOrderDetail(c *gin.Context) {
	orderID, err := orderIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	order, err := h.svc.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, "order retrieved successfully", order)
}

// updateStatusRequest menerima status sebagai string ("preparing"), bukan
// angka. Kontrak API tetap terbaca meski di database SMALLINT.
type updateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateStatus PATCH /api/v1/orders/:id/status
func (h *OrdersController) UpdateStatus(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		_ = c.Error(exception.Unauthorized("AUTH_401", "authentication required"))
		return
	}

	orderID, err := orderIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(exception.Validation(err))
		return
	}

	next, err := config.ParseOrderStatus(req.Status)
	if err != nil {
		_ = c.Error(exception.BadRequest("ORDER_400", "unknown order status"))
		return
	}

	order, err := h.svc.UpdateStatus(c.Request.Context(), orderID, next, userID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response.OK(c, "order status updated", order)
}

// ============================================================================
// Helper
// ============================================================================

func orderIDParam(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, exception.BadRequest("ORDER_400", "invalid order id")
	}
	return id, nil
}

func idempotencyKey(c *gin.Context) (string, error) {
	key := c.GetHeader(idempotencyHeader)
	if len(key) > maxIdempotencyKeyLength {
		return "", exception.BadRequest("ORDER_400", "idempotency key is too long")
	}
	return key, nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}