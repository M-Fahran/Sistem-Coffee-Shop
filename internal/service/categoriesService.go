package service

import (
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"context"
	"fmt"
)

type CategoriesRequest struct {
	Name string `json:"name" binding:"required"`
}

// type UpdateCategoriesRequest struct {
// 	Name string `json:"name" binding:"required"`
// }

type CategoriesService struct {
	repo *repository.CategoriesRepository
}

func NewCategoriesService(repo *repository.CategoriesRepository) *CategoriesService {
	return &CategoriesService{repo: repo}
}

func (s *CategoriesService) GetAllCategories(ctx context.Context) ([]entity.Categories, error) {
	categories, err := s.repo.GetAllCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar kategori: %w", err)
	}
	return categories, nil
}

func (s *CategoriesService) CreateCategories(ctx context.Context, req CategoriesRequest) (*entity.Categories, error) {
	categories := &entity.Categories{
		Name: req.Name,
	}
	err := s.repo.Create(ctx, categories)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat kategori baru: %w", err)
	}
	return categories, nil
}

func (s *CategoriesService) UpdateCategories(ctx context.Context, id int64, req CategoriesRequest) error {
	categories := &entity.Categories{
		ID: id,
		Name: req.Name,
	}

	err := s.repo.Update(ctx, categories)
	if err != nil {
		return fmt.Errorf("gagal mengupdate kategori (ID: %d) : %w", id, err)
	}

	return nil
}

func (s *CategoriesService) DeleteCategories(ctx context.Context, id int64) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus kategori (ID: %d): %w", id, err)
	}
	return nil
}