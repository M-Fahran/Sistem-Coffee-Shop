package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/support/cache"
	"coffeeshop/internal/support/exception"
)

// providerInternal dipakai untuk pembayaran tunai — tidak ada gateway yang
// terlibat, kasir yang menerima uangnya.
const providerInternal = "Internal"

type OrdersService struct {
	repo *repository.OrdersRepository
	idem *cache.IdempotencyStore
	log  *slog.Logger

	// gatewayName masuk ke kolom provider untuk pembayaran non-tunai.
	// Nanti diganti nama gateway sungguhan saat integrasi.
	gatewayName string
}

func NewOrdersService(
	repo *repository.OrdersRepository,
	idem *cache.IdempotencyStore,
	log *slog.Logger,
	gatewayName string,
) *OrdersService {
	return &OrdersService{
		repo:        repo,
		idem:        idem,
		log:         log,
		gatewayName: gatewayName,
	}
}

// ============================================================================
// Read
// ============================================================================

func (s *OrdersService) GetAllOrders(ctx context.Context) ([]entity.Order, error) {
	return s.repo.GetAllOrders(ctx)
}

func (s *OrdersService) GetOrderByID(ctx context.Context, orderID int64) (entity.Order, error) {
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return entity.Order{}, err
	}

	items, err := s.repo.GetItemsByOrderID(ctx, orderID)
	if err != nil {
		return entity.Order{}, err
	}
	order.Items = items
	return order, nil
}

// GetTableOrder mengambil pesanan dengan syarat pesanan itu milik meja yang
// sedang aktif di session.
//
// Tanpa pengecekan ini, pelanggan meja 3 bisa membaca pesanan meja 7 hanya
// dengan menebak ID — termasuk nama pelanggan dan isi pesanannya.
func (s *OrdersService) GetTableOrder(ctx context.Context, orderID, tableID int64) (entity.Order, error) {
	order, err := s.GetOrderByID(ctx, orderID)
	if err != nil {
		return entity.Order{}, err
	}

	if order.TableID == nil || *order.TableID != tableID {
		// Sengaja 404, bukan 403 — menjawab "ada tapi bukan milikmu" sama
		// saja membenarkan bahwa ID itu valid.
		return entity.Order{}, exception.NotFound("ORDER_404", "order not found")
	}
	return order, nil
}

// ============================================================================
// Create
// ============================================================================

// CreateCustomerOrder membuat pesanan dari pelanggan yang scan QR.
//
// tableID datang dari session token, bukan dari body request.
func (s *OrdersService) CreateCustomerOrder(
	ctx context.Context,
	tableID int64,
	idempotencyKey string,
	in entity.CreateOrderInput,
) (*entity.Order, error) {
	in.TableID = &tableID
	in.Source = config.OrderSourceQR
	in.CreatedByUserID = nil // pesanan QR tidak punya kasir pencatat
	in.Provider = s.providerFor(in.PaymentMethod)

	scope := fmt.Sprintf("table:%d", tableID)
	return s.create(ctx, scope, idempotencyKey, in)
}

// CreateCashierOrder membuat pesanan yang dicatat kasir di counter.
func (s *OrdersService) CreateCashierOrder(
	ctx context.Context,
	userID int64,
	tableID *int64,
	idempotencyKey string,
	in entity.CreateOrderInput,
) (*entity.Order, error) {
	in.TableID = tableID
	in.Source = config.OrderSourceCashier
	in.CreatedByUserID = &userID
	in.Provider = s.providerFor(in.PaymentMethod)

	scope := fmt.Sprintf("user:%d", userID)
	return s.create(ctx, scope, idempotencyKey, in)
}

// create menjalankan pembuatan pesanan dengan penjagaan idempotency.
//
// Idempotency key bersifat opsional. Kalau klien tidak mengirimnya, pesanan
// tetap dibuat — hanya saja retry akan menghasilkan pesanan ganda, dan itu
// tanggung jawab klien.
func (s *OrdersService) create(
	ctx context.Context,
	scope, idempotencyKey string,
	in entity.CreateOrderInput,
) (*entity.Order, error) {
	if idempotencyKey == "" {
		return s.repo.CreateOrder(ctx, in)
	}

	existingID, done, err := s.idem.Claim(ctx, scope, idempotencyKey)
	switch {
	case errors.Is(err, cache.ErrInProgress):
		return nil, exception.Conflict("ORDER_409", "an identical request is still being processed")
	case err != nil:
		// Redis bermasalah bukan alasan menolak pesanan. Risikonya pesanan
		// ganda kalau klien retry — jauh lebih ringan daripada kedai tidak
		// bisa menerima pesanan sama sekali.
		s.log.Warn("idempotency claim failed, proceeding without guard",
			"scope", scope, "err", err)
		return s.repo.CreateOrder(ctx, in)
	case done:
		// Request kembar yang sudah selesai — kembalikan pesanan yang sama.
		order, err := s.GetOrderByID(ctx, existingID)
		if err != nil {
			return nil, err
		}
		return &order, nil
	}

	order, err := s.repo.CreateOrder(ctx, in)
	if err != nil {
		// Lepas klaim supaya klien bisa langsung memperbaiki pesanan dan
		// mencoba lagi dengan key yang sama.
		if releaseErr := s.idem.Release(ctx, scope, idempotencyKey); releaseErr != nil {
			s.log.Warn("idempotency release failed", "scope", scope, "err", releaseErr)
		}
		return nil, err
	}

	if err := s.idem.Commit(ctx, scope, idempotencyKey, order.ID); err != nil {
		// Pesanan sudah terlanjur dibuat dan valid. Kegagalan mencatat hasil
		// hanya berarti retry berikutnya bisa membuat pesanan ganda.
		s.log.Warn("idempotency commit failed", "order_id", order.ID, "err", err)
	}

	s.log.Info("order created",
		"order_id", order.ID,
		"order_number", order.OrderNumber,
		"source", order.Source.String(),
		"subtotal", order.Subtotal,
	)
	return order, nil
}

// ============================================================================
// Status
// ============================================================================

// UpdateStatus memindahkan pesanan ke status berikutnya.
//
// Keabsahan transisi diperiksa di dalam lock database (lihat repository),
// jadi dua kasir yang menekan tombol bersamaan tidak bisa menghasilkan
// perpindahan ganda.
func (s *OrdersService) UpdateStatus(
	ctx context.Context,
	orderID int64,
	next config.OrderStatus,
	actorUserID int64,
) (*entity.Order, error) {
	order, err := s.repo.UpdateStatus(ctx, orderID, next)
	if err != nil {
		return nil, err
	}

	s.log.Info("order status changed",
		"order_id", order.ID,
		"order_number", order.OrderNumber,
		"status", order.Status.String(),
		"by_user_id", actorUserID,
	)
	return order, nil
}

// providerFor menentukan isi kolom provider.
func (s *OrdersService) providerFor(method config.PaymentMethod) string {
	if method.IsOnline() {
		return s.gatewayName
	}
	return providerInternal
}