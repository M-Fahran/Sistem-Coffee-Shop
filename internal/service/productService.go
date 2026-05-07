package service

import (
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/request"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type ProductService struct {
	productRepo *repository.ProductRepository
	categoryRepo *repository.CategoriesRepository
	redisClient *redis.Client
}

type ProductAddOnService struct {
	repo *repository.ProductAddOnRepository
}

func NewProductService(
	productRepo *repository.ProductRepository, 
	categoryRepo *repository.CategoriesRepository,
	redisClient *redis.Client) *ProductService {
	return &ProductService{
		productRepo: productRepo,
		categoryRepo: categoryRepo,
		redisClient: redisClient,
	}
}

func NewProductAddOnService(repo *repository.ProductAddOnRepository) *ProductAddOnService {
	return &ProductAddOnService{repo: repo}
}

func (s *ProductService) GetAllProducts(ctx context.Context, filterStatus, categoryID string) ([]entity.Product, error) {
	cacheKey := fmt.Sprintf("products:status:%s:category:%s", filterStatus, categoryID)
	cacheData, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var products []entity.Product
		if err := json.Unmarshal([]byte(cacheData), &products); err == nil {
			fmt.Println("ambil data produk dari redis cache")
			return products, nil
		}
	} else if err != redis.Nil {
		fmt.Printf("gagal membaca redis: %v\n", err)
	}
	fmt.Println("CACHE MISS")

	products, err := s.productRepo.GetAllProduct(ctx, filterStatus, categoryID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produk: %w", err)
	}

	for i := range products {
		addon, err := s.productRepo.GetAddOnByProductID(ctx, products[i].ID)
		if err != nil {
			products[i].AddOn = addon
		}
	}

	productJSON, err := json.Marshal(products)
	if err == nil {
		err = s.redisClient.Set(ctx, cacheKey, productJSON, 5*time.Minute).Err()
		if err != nil {
			fmt.Printf("gagal menyimpan cache redis: %v\n", err)
		}
	}
	return products, nil
}

func (s *ProductService) CreateProduct(ctx context.Context, req request.CreateProductRequest) (int64, error) {
	_, err := s.categoryRepo.GetCategoriesByID(ctx, req.CategoryID)
	if err != nil {
		return 0, fmt.Errorf("kategori dengan ID %d tidak valid/tidak ditemukan", req.CategoryID)
	}
	
	product := entity.Product{
		CategoryID: req.CategoryID,
		Name:       req.Name,
		BasePrice:  req.Price,
		Stock:      req.Stock,
		IsActive:   true,
	}
	
	newID, err := s.productRepo.CreateProduct(ctx, &product)
	if err != nil {
		return 0, fmt.Errorf("gagal menyimpan produk ke database: %w", err)
	}

	if len(req.AddonID) > 0 {
		err = s.productRepo.SyncProductAddOn(ctx, newID, req.AddonID)
		if err != nil {
			return newID, fmt.Errorf("produk berhasil dibuat namun gagal menambahkan addOn: %w", err)
		}
	}

	return newID, nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, id int64, req request.UpdateProductRequest) error {
	existingProduct, err := s.productRepo.GetProductByID(ctx, id)
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

	err = s.productRepo.UpdateProduct(ctx, &existingProduct)

	if err != nil {
		return fmt.Errorf("gagal mengupdate produk (ID: %d) : %w", id, err)
	}

	if req.AddonID != nil {
		err = s.productRepo.SyncProductAddOn(ctx, id, req.AddonID)
		if err != nil {
			return fmt.Errorf("gagal sinkron addOn: %w", err)
		}
	}

	return nil
}

func (s *ProductService) DeleteProduct(ctx context.Context, id int64) error {
	err := s.productRepo.DeleteProduct(ctx, id)
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

func (s *ProductAddOnService) CreateProductAddOn(ctx context.Context, req request.CreateProductAddOnRequest) (*entity.ProductAddon, error) {
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

func (s *ProductAddOnService) UpdateProductAddOn(ctx context.Context, id int64, req request.UpdateProductAddOnRequest) error {
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
