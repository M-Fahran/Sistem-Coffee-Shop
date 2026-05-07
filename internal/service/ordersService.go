package service

import (
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"context"
	"fmt"
)

type OrdersService struct {
	repo *repository.OrdersRepository
}

func NewOrdersService(repo *repository.OrdersRepository) *OrdersService {
	return &OrdersService{repo: repo}
}

func (s *OrdersService) GetAllOrders(ctx context.Context) ([]entity.Orders, error) {
	products, err := s.repo.GetAllOrders(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produk: %w", err)
	}
	return products, nil
}

func (s *OrdersService) GetOrderByID(ctx context.Context, orderID int64) (entity.Orders, error){
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return entity.Orders{}, fmt.Errorf("order tidak ditemukan: %w", err)
	}

	items, err := s.repo.GetItemsByOrderID(ctx, orderID)
	if err != nil {
		return entity.Orders{}, fmt.Errorf("GAGAL TARIK ITEMS: %v", err)
	}
	order.Items = items

	return order, nil
} 