package service

import (
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/request"
	"coffeeshop/internal/support/cache"
	"context"
	"fmt"
	"log"
)

type ProductService struct {
	productRepo  *repository.ProductRepository
	categoryRepo *repository.CategoriesRepository
	redisCache   *cache.ProductCache
}

type ProductAddOnService struct {
	repo *repository.ProductAddOnRepository
}

func NewProductService(
	productRepo *repository.ProductRepository,
	categoryRepo *repository.CategoriesRepository,
	redisCache *cache.ProductCache,
) *ProductService {
	return &ProductService{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		redisCache:   redisCache,
	}
}

func NewProductAddOnService(repo *repository.ProductAddOnRepository) *ProductAddOnService {
	return &ProductAddOnService{repo: repo}
}

func (s *ProductService) GetAllProducts(ctx context.Context, filterStatus, categoryID string) ([]entity.Product, error) {
	products, err := s.redisCache.GetAll(ctx, filterStatus, categoryID)
	if err == nil && len(products) > 0 {
		log.Println("data diambil dari cache")
		return products, nil
	} else if err != nil {
		log.Printf("info cache: %v\n", err)
	}

	log.Println("data diambil dari db")
	products, err = s.productRepo.GetAllProduct(ctx, filterStatus, categoryID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar produk: %w", err)
	}

	for i := range products {
		addon, err := s.productRepo.GetAddOnByProductID(ctx, products[i].ID)
		if err != nil {
			products[i].AddOn = addon
		}
	}

	err = s.redisCache.SetAll(ctx, filterStatus, categoryID, products)
	if err != nil {
		log.Println("gagal menyimpan ke cache:", err)
	}
	return products, nil
}

func (s *ProductService) GetProductByID(ctx context.Context, id int64) (entity.Product, error){
	// cacheproduct, err := s.redisCache.GetByID(ctx, id)
	// if err == nil && cacheproduct != nil {
	// 	log.Println("data diambil dari cache")
	// 	return *cacheproduct, nil
	// } else if err != nil {
	// 	log.Printf("info cache: %v\n", err)
	// }

	log.Println("data diambil dari db")
	product, err := s.productRepo.GetProductByID(ctx, id)
	if err != nil {
		return entity.Product{}, fmt.Errorf("produk tidak ditemukan: %w", err)
	}

	// err = s.redisCache.SetByID(ctx, &product)
	// if err != nil {
	// 	log.Printf("gagal menyimpan ke cache: %v\n", err)
	// }

	return product, nil
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

	err = s.redisCache.InvalidDateAll(ctx)
	if err != nil {
		log.Println("gagal hapus cache")
	} else {
		log.Println("berhasil hapus cache karena ada data baru")
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

	err = s.redisCache.InvaliDateByID(ctx, id)
	if err != nil {
		log.Printf("gagal hapus cache produk id %d: %v\n", id, err)
	}

	err = s.redisCache.InvalidDateAll(ctx)
	if err != nil {
		log.Printf("gagal menghapus cache produk all: %d: %v\n", err)
	}

	if req.AddonID != nil {
		err = s.productRepo.SyncProductAddOn(ctx, id, req.AddonID)
		if err != nil {
			return fmt.Errorf("gagal sinkron addOn: %w", err)
		}
	}
	
	log.Println("cache produk berhasil")
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
