package service

import (
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"context"
	"errors"
)

type CreateProductRequest struct {
	CategoryID int `json:"category_id" binding:"required"`
	Name string `json:"name" binding:"required"`
	BasePrice int `json:"base_price" binding:"required,gte=0"`
	Stock int `json:"stock" binding:"required,gte=0"`
}

type ProductService struct {
	repo *repository.ProductRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) CreateProduct(ctx context.Context, req CreateProductRequest) (*entity.Product, error) {
	product := &entity.Product{
		CategoryID: req.CategoryID,
		Name: req.Name,
		BasePrice: req.BasePrice,
		Stock: req.Stock,
		IsActive: true,
	}
	err := s.repo.Create(ctx, product)
	if err != nil {
		return nil, errors.New("gagal menyimpan produk ke database")
	}
	return product, nil
}