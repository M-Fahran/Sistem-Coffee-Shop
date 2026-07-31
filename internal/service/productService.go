package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/request"
)

const productCacheTTL = 5 * time.Minute

// productCachePrefix dipakai untuk menghapus seluruh varian cache produk
// sekaligus saat ada perubahan data.
const productCachePrefix = "products:"

type ProductService struct {
	productRepo  *repository.ProductRepository
	categoryRepo *repository.CategoriesRepository
	redisClient  *redis.Client
}

type ProductAddOnService struct {
	repo *repository.ProductAddOnRepository
}

func NewProductService(
	productRepo *repository.ProductRepository,
	categoryRepo *repository.CategoriesRepository,
	redisClient *redis.Client,
) *ProductService {
	return &ProductService{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		redisClient:  redisClient,
	}
}

func NewProductAddOnService(repo *repository.ProductAddOnRepository) *ProductAddOnService {
	return &ProductAddOnService{repo: repo}
}

// ============================================================================
// Read
// ============================================================================

func (s *ProductService) GetAllProducts(ctx context.Context, filterStatus, categoryID string) ([]entity.Product, error) {
	cacheKey := fmt.Sprintf("%sstatus:%s:category:%s", productCachePrefix, filterStatus, categoryID)

	if cached, err := s.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
		var products []entity.Product
		if err := json.Unmarshal(cached, &products); err == nil {
			return products, nil
		}
		// Cache rusak — buang, lalu ambil dari database.
		_ = s.redisClient.Del(ctx, cacheKey).Err()
	} else if !errors.Is(err, redis.Nil) {
		slog.Warn("product cache read failed", "key", cacheKey, "err", err)
	}

	products, err := s.productRepo.GetAllProduct(ctx, filterStatus, categoryID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produk: %w", err)
	}

	// BUG LAMA: kondisinya terbalik — `if err != nil { products[i].AddOn = addon }`.
	// Add-on hanya di-assign ketika query GAGAL, jadi menu tidak pernah
	// membawa add-on sama sekali.
	for i := range products {
		addons, err := s.productRepo.GetAddOnByProductID(ctx, products[i].ID)
		if err != nil {
			return nil, fmt.Errorf("gagal mengambil add-on produk %d: %w", products[i].ID, err)
		}
		products[i].Addons = addons
	}

	if payload, err := json.Marshal(products); err == nil {
		if err := s.redisClient.Set(ctx, cacheKey, payload, productCacheTTL).Err(); err != nil {
			slog.Warn("product cache write failed", "key", cacheKey, "err", err)
		}
	}

	return products, nil
}

// ============================================================================
// Write
// ============================================================================

func (s *ProductService) CreateProduct(ctx context.Context, req request.CreateProductRequest) (int64, error) {
	if _, err := s.categoryRepo.GetCategoriesByID(ctx, req.CategoryID); err != nil {
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
		if err := s.productRepo.SyncProductAddOn(ctx, newID, req.AddonID); err != nil {
			return newID, fmt.Errorf("produk berhasil dibuat namun gagal menambahkan addOn: %w", err)
		}
	}

	s.invalidateCache(ctx)
	return newID, nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, id int64, req request.UpdateProductRequest) error {
	existing, err := s.productRepo.GetProductByID(ctx, id)
	if err != nil {
		return fmt.Errorf("produk dengan ID %d tidak ditemukan: %w", id, err)
	}

	if req.CategoryID != nil {
		existing.CategoryID = *req.CategoryID
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Price != nil {
		existing.BasePrice = *req.Price
	}
	if req.Stock != nil {
		existing.Stock = *req.Stock
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := s.productRepo.UpdateProduct(ctx, &existing); err != nil {
		return fmt.Errorf("gagal mengupdate produk (ID: %d): %w", id, err)
	}

	if req.AddonID != nil {
		if err := s.productRepo.SyncProductAddOn(ctx, id, req.AddonID); err != nil {
			return fmt.Errorf("gagal sinkron addOn: %w", err)
		}
	}

	s.invalidateCache(ctx)
	return nil
}

func (s *ProductService) DeleteProduct(ctx context.Context, id int64) error {
	if err := s.productRepo.DeleteProduct(ctx, id); err != nil {
		return fmt.Errorf("gagal menghapus produk (ID: %d): %w", id, err)
	}
	s.invalidateCache(ctx)
	return nil
}

// invalidateCache membuang seluruh varian cache daftar produk.
//
// Tanpa ini, admin mengubah harga tapi pelanggan masih melihat harga lama
// sampai TTL habis — persis masalah yang ada sebelumnya.
//
// SCAN dipakai (bukan KEYS) supaya tidak memblokir Redis. Jumlah key di sini
// kecil (kombinasi status × kategori), jadi biayanya tidak terasa.
func (s *ProductService) invalidateCache(ctx context.Context) {
	iter := s.redisClient.Scan(ctx, 0, productCachePrefix+"*", 100).Iterator()

	keys := make([]string, 0, 16)
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		slog.Warn("product cache scan failed", "err", err)
		return
	}
	if len(keys) == 0 {
		return
	}
	if err := s.redisClient.Del(ctx, keys...).Err(); err != nil {
		slog.Warn("product cache invalidate failed", "count", len(keys), "err", err)
	}
}

// ============================================================================
// Add-on
// ============================================================================

func (s *ProductAddOnService) GetAllProductsAddOn(ctx context.Context) ([]entity.ProductAddon, error) {
	addons, err := s.repo.GetAllProductAddOn(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produkAddOn: %w", err)
	}
	return addons, nil
}

func (s *ProductAddOnService) CreateProductAddOn(ctx context.Context, req request.CreateProductAddOnRequest) (*entity.ProductAddon, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	addon := &entity.ProductAddon{
		Name:     req.Name,
		Price:    req.Price,
		Stock:    req.Stock,
		IsActive: isActive,
	}
	if err := s.repo.CreateProductAddOn(ctx, addon); err != nil {
		return nil, fmt.Errorf("gagal menyimpan add-on ke database: %w", err)
	}
	return addon, nil
}

func (s *ProductAddOnService) UpdateProductAddOn(ctx context.Context, id int64, req request.UpdateProductAddOnRequest) error {
	existing, err := s.repo.GetProductAddOnByID(ctx, id)
	if err != nil {
		return fmt.Errorf("add-on dengan ID %d tidak ditemukan: %w", id, err)
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Price != nil {
		existing.Price = *req.Price
	}
	if req.Stock != nil {
		existing.Stock = *req.Stock
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := s.repo.UpdateProductAddOn(ctx, &existing); err != nil {
		return fmt.Errorf("gagal mengupdate add-on (ID: %d): %w", id, err)
	}
	return nil
}

func (s *ProductAddOnService) DeleteProductAddOn(ctx context.Context, id int64) error {
	if err := s.repo.DeleteProductAddOn(ctx, id); err != nil {
		return fmt.Errorf("gagal menghapus add on (ID: %d): %w", id, err)
	}
	return nil
}