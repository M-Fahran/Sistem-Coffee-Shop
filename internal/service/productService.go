package service

import (
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"context"
	"fmt"
)

type CreateProductRequest struct {
	CategoryID int64   `json:"category_id" binding:"required"`
	Name       string  `json:"name" binding:"required"`
	Price      int     `json:"price" binding:"required,gte=0"`
	Stock      int     `json:"stock" binding:"required,gte=0"`
	AddonID    []int64 `json:"addon_id"`
}

type UpdateProductRequest struct {
	CategoryID *int64  `json:"category_id"`
	Name       *string `json:"name"`
	Price      *int    `json:"price"`
	Stock      *int    `json:"stock"`
	Is_Active  *bool   `json:"is_active"`
	AddonID    []int64 `json:"addon_id"`
}

type CreateProductAddOnRequest struct {
	Name     string `json:"name" binding:"required"`
	Price    int    `json:"price" binding:"required,gte=0"`
	Stock    int    `json:"stock" binding:"required,gte=0"`
	IsActive *bool  `json:"is_active" binding:"required"`
}

type UpdateProductAddOnRequest struct {
	Name      *string `json:"name"`
	Price     *int    `json:"price"`
	Stock     *int    `json:"stock"`
	Is_Active *bool   `json:"is_active"`
}

type ProductService struct {
	repo *repository.ProductRepository
}

type ProductAddOnService struct {
	repo *repository.ProductAddOnRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func NewProductAddOnService(repo *repository.ProductAddOnRepository) *ProductAddOnService {
	return &ProductAddOnService{repo: repo}
}

func (s *ProductService) GetAllProducts(ctx context.Context, filterStatus, categoryID string) ([]entity.Product, error) {
	products, err := s.repo.GetAllProduct(ctx, filterStatus, categoryID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produk: %w", err)
	}

	for i := range products {
		addon, err := s.repo.GetAddOnByProductID(ctx, products[i].ID)
		if err != nil {
			products[i].AddOn = addon
		}
	}
	return products, nil
}

func (s *ProductService) CreateProduct(ctx context.Context, req CreateProductRequest) (int64, error) {
	product := entity.Product{
		CategoryID: req.CategoryID,
		Name:       req.Name,
		BasePrice:  req.Price,
		Stock:      req.Stock,
		IsActive:   true,
	}
	
	newID, err := s.repo.CreateProduct(ctx, &product)
	if err != nil {
		return 0, fmt.Errorf("gagal menyimpan produk ke database: %w", err)
	}

	if len(req.AddonID) > 0 {
		err = s.repo.SyncProductAddOn(ctx, newID, req.AddonID)
		if err != nil {
			return newID, fmt.Errorf("produk berhasil dibuat namun gagal menambahkan addOn: %w", err)
		}
	}

	return newID, nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, id int64, req UpdateProductRequest) error {
	existingProduct, err := s.repo.GetProductByID(ctx, id)
	if err != nil {
		return fmt.Errorf("produk dengan ID %d tidak ditemukan: %w", id, err)
	}

	if req.CategoryID != nil {
		existingProduct.CategoryID = *req.CategoryID
	}

	if req.Name != nil {
		existingProduct.Name = *req.Name
	}

	if req.Price != nil {
		existingProduct.BasePrice = *req.Price
	}

	if req.Stock != nil {
		existingProduct.Stock = *req.Stock
	}

	if req.Is_Active != nil {
		existingProduct.IsActive = *req.Is_Active
	}

	err = s.repo.UpdateProduct(ctx, &existingProduct)

	if err != nil {
		return fmt.Errorf("gagal mengupdate produk (ID: %d) : %w", id, err)
	}

	if req.AddonID != nil {
		err = s.repo.SyncProductAddOn(ctx, id, req.AddonID)
		if err != nil {
			return fmt.Errorf("gagal sinkron addOn: %w", err)
		}
	}

	return nil
}

func (s *ProductService) DeleteProduct(ctx context.Context, id int64) error {
	err := s.repo.DeleteProduct(ctx, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus produk (ID: %d): %w", id, err)
	}
	return nil
}

func (s *ProductAddOnService) GetAllProductsAddOn(ctx context.Context) ([]entity.ProductAddon, error) {
	productsAddOn, err := s.repo.GetAllProductAddOn(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produkAddOn: %w", err)
	}
	return productsAddOn, nil
}

func (s *ProductAddOnService) CreateProductAddOn(ctx context.Context, req CreateProductAddOnRequest) (*entity.ProductAddon, error) {
	productAddOn := &entity.ProductAddon{
		Name:     req.Name,
		Price:    req.Price,
		Stock:    req.Stock,
		IsActive: true,
	}
	err := s.repo.CreateProductAddOn(ctx, productAddOn)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan produk ke database: %w", err)
	}
	return productAddOn, nil
}

func (s *ProductAddOnService) UpdateProductAddOn(ctx context.Context, id int64, req UpdateProductAddOnRequest) error {
	existingProduct, err := s.repo.GetProductAddOnByID(ctx, id)
	if err != nil {
		return fmt.Errorf("produk dengan ID %d tidak ditemukan: %w", id, err)
	}

	if req.Name != nil {
		existingProduct.Name = *req.Name
	}

	if req.Price != nil {
		existingProduct.Price = *req.Price
	}

	if req.Stock != nil {
		existingProduct.Stock = *req.Stock
	}

	if req.Is_Active != nil {
		existingProduct.IsActive = *req.Is_Active
	}

	err = s.repo.UpdateProductAddOn(ctx, &existingProduct)

	if err != nil {
		return fmt.Errorf("gagal mengupdate produk (ID: %d) : %w", id, err)
	}

	return nil
}

func (s *ProductAddOnService) DeleteProductAddOn(ctx context.Context, id int64) error {
	err := s.repo.DeleteProductAddOn(ctx, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus add on (ID: %d): %w", id, err)
	}
	return nil
}
