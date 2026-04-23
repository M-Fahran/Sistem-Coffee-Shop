package service

import (
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"context"
	"fmt"
)

type CreateProductRequest struct {
	CategoryID int64  `json:"category_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	BasePrice  int    `json:"base_price" binding:"required,gte=0"`
	Stock      int    `json:"stock" binding:"required,gte=0"`
}
type UpdateProductRequest struct {
	CategoryID int64  `json:"category_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	BasePrice  int    `json:"base_price" binding:"required,gte=0"`
	Stock      int    `json:"stock" binding:"required,gte=0"`
	Is_Active  *bool `json:"is_active" binding:"required"`
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
		Name:       req.Name,
		BasePrice:  req.BasePrice,
		Stock:      req.Stock,
		IsActive:   true,
	}
	err := s.repo.Create(ctx, product)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan produk ke database: %w", err)
	}
	return product, nil
}

func (s *ProductService) GetAllActiveProducts(ctx context.Context) ([]entity.Product, error) {
	products, err := s.repo.GetAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produk aktif: %w", err)
	}
	return products, nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, id int64, req UpdateProductRequest) error {
	product := &entity.Product{
		ID: id,
		CategoryID: req.CategoryID,
		Name: req.Name,
		BasePrice: req.BasePrice,
		Stock: req.Stock,
		IsActive: *req.Is_Active,
	}

	err := s.repo.Update(ctx, product)
	if err != nil {
		return fmt.Errorf("gagal mengupdate produk (ID: %d) : %w", id, err)
	}

	return nil
}
