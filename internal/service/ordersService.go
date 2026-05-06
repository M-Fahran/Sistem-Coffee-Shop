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