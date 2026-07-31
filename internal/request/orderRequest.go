package request

import (
	"strings"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/support/exception"
)

// Batas ukuran pesanan. Bukan aturan bisnis, tapi pagar terhadap request
// yang sengaja dibesarkan untuk membebani database — satu request bisa
// mengunci ratusan baris produk kalau tidak dibatasi.
const (
	maxOrderDistinctItems  = 30
	maxOrderTotalQuantity  = 200
	maxOrderAddonsPerItem  = 10
	maxOrderQuantityPerRow = 50
	maxOrderNotesLength    = 255
)

// CreateOrderRequest adalah body untuk pelanggan maupun kasir.
//
// Tidak ada field harga dan tidak ada table_id: harga ditentukan server,
// meja ditentukan session (pelanggan) atau field terpisah (kasir).
type CreateOrderRequest struct {
	CustomerName  *string         `json:"customer_name"  binding:"omitempty,max=100"`
	CustomerEmail *string         `json:"customer_email" binding:"omitempty,email,max=255"`
	PaymentMethod string          `json:"payment_method" binding:"required"`
	Items         []OrderItemBody `json:"items"          binding:"required,min=1,dive"`
}

type OrderItemBody struct {
	ProductID int64   `json:"product_id" binding:"required,gt=0"`
	Quantity  int     `json:"quantity"   binding:"required,gt=0"`
	Notes     *string `json:"notes"      binding:"omitempty,max=255"`
	AddonIDs  []int64 `json:"addon_ids"  binding:"omitempty,dive,gt=0"`
}

// CashierOrderRequest menambahkan table_id, karena kasir memilih meja
// secara manual (atau tidak sama sekali untuk take away).
type CashierOrderRequest struct {
	CreateOrderRequest
	TableID *int64 `json:"table_id" binding:"omitempty,gt=0"`
}

func BindCreateOrderRequest(c *gin.Context) (*CreateOrderRequest, error) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, exception.Validation(err)
	}
	req.prepare()
	if err := req.validate(); err != nil {
		return nil, err
	}
	return &req, nil
}

func BindCashierOrderRequest(c *gin.Context) (*CashierOrderRequest, error) {
	var req CashierOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, exception.Validation(err)
	}
	req.prepare()
	if err := req.validate(); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *CreateOrderRequest) prepare() {
	r.PaymentMethod = strings.ToLower(strings.TrimSpace(r.PaymentMethod))

	if r.CustomerName != nil {
		trimmed := strings.TrimSpace(*r.CustomerName)
		if trimmed == "" {
			r.CustomerName = nil
		} else {
			r.CustomerName = &trimmed
		}
	}
	if r.CustomerEmail != nil {
		trimmed := strings.ToLower(strings.TrimSpace(*r.CustomerEmail))
		if trimmed == "" {
			r.CustomerEmail = nil
		} else {
			r.CustomerEmail = &trimmed
		}
	}

	for i := range r.Items {
		if r.Items[i].Notes == nil {
			continue
		}
		trimmed := strings.TrimSpace(*r.Items[i].Notes)
		if trimmed == "" {
			r.Items[i].Notes = nil
			continue
		}
		if len(trimmed) > maxOrderNotesLength {
			trimmed = trimmed[:maxOrderNotesLength]
		}
		r.Items[i].Notes = &trimmed
	}
}

func (r *CreateOrderRequest) validate() error {
	if len(r.Items) > maxOrderDistinctItems {
		return exception.BadRequest("ORDER_400", "too many items in one order")
	}

	if _, err := config.ParsePaymentMethod(r.PaymentMethod); err != nil {
		return exception.BadRequest("ORDER_400", "unsupported payment method")
	}

	total := 0
	for _, item := range r.Items {
		if item.Quantity > maxOrderQuantityPerRow {
			return exception.BadRequest("ORDER_400", "quantity per item is too large")
		}
		if len(item.AddonIDs) > maxOrderAddonsPerItem {
			return exception.BadRequest("ORDER_400", "too many add-ons for one item")
		}
		if hasDuplicateIDs(item.AddonIDs) {
			return exception.BadRequest("ORDER_400", "duplicate add-on in one item")
		}
		total += item.Quantity
	}

	if total > maxOrderTotalQuantity {
		return exception.BadRequest("ORDER_400", "total quantity is too large")
	}
	return nil
}

// ToServiceInput menerjemahkan request jadi input service.
//
// tableID, source, dan createdByUserID datang dari controller (session atau
// token), BUKAN dari body — itu yang mencegah pelanggan memesan atas nama
// meja lain atau mengaku sebagai kasir.
func (r *CreateOrderRequest) ToServiceInput(
	tableID *int64,
	source config.OrderSource,
	createdByUserID *int64,
) entity.CreateOrderInput {
	// Sudah divalidasi di validate(), jadi error di sini tidak mungkin.
	method, _ := config.ParsePaymentMethod(r.PaymentMethod)

	items := make([]entity.OrderItemInput, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, entity.OrderItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Notes:     item.Notes,
			AddonIDs:  item.AddonIDs,
		})
	}

	return entity.CreateOrderInput{
		TableID:         tableID,
		CustomerName:    r.CustomerName,
		CustomerEmail:   r.CustomerEmail,
		Source:          source,
		CreatedByUserID: createdByUserID,
		PaymentMethod:   method,
		Items:           items,
	}
}

func hasDuplicateIDs(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}