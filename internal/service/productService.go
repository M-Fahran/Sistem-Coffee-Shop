package service

import (
	"context"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
)

type ProductService interface {
	GetAvailableMenu(ctx context.Context) ([]entity.Product, error)
}

type productService struct {
	productRepo repository.ProductRepository
}

func NewProductService(repo repository.ProductRepository) ProductService {
	return &productService{
		productRepo: repo,
	}
}

func (s *productService) GetAvailableMenu(ctx context.Context) ([]entity.Product, error) {
	products, err := s.productRepo.GetAllActiveProducts(ctx)
	if err != nil {
		return nil, err
	}

	return products, nil
}