package service 
import (
	"context"
	"fmt"
	"log/slog"

	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/support/qrtoken"
	"coffeeshop/internal/support/cache"
)


type TableService struct {
	repo *repository.TableRepository
	cache *cache.TableCache
	log *slog.Logger
}

func NewTableService(
	repo *repository.TableRepository,
	cache *cache.TableCache,
	log *slog.Logger,
) *TableService {
	return &TableService{repo:repo, cache:cache, log:log}
}

// read list of 
func (s *TableService) List(ctx context.Context, in entity.ListTablesInput)(*entity.ListResult, error){
	items, total, err := s.repo.List(ctx, in)
	if err != nil{
		return nil, err
	}
	return &entity.ListResult{
		Items: items, 
		Total: total,
		Page: in.Page,
		PerPage: in.PerPage,
	}, nil
}
// read get by id 
func (s *TableService) GetByID(ctx context.Context, id int64) (*entity.Table, error){
	if cached, err := s.cache.GetByID(ctx, id); err != nil {
		s.log.Warn("cache get by id failed", "id", id, "err", err)
	} else if cached != nil {
		return cached, nil
	}

	t, err := s.repo.FindByID(ctx, id)
	if err != nil{
		return nil, err
	}

	if err := s.cache.SetByID(ctx, t); err != nil {
		s.log.Warn("cache set by id failed", "id", id, "err", err)
	}

	return t, nil
}

//create a new table
func (s *TableService) Create(ctx context.Context, in entity.CreateTableInput) (*entity.Table, error){
	token, err:= qrtoken.GenerateQRToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR token: %w", err)
	}
	t := &entity.Table{
		Number:   in.Number,
		QRToken:  token,
		IsActive: in.IsActive,
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err // already *exception.Exception, just forward
	}

	s.log.Info("table created", "id", t.ID, "number", t.Number)
	return t, nil
}

// update 
func (s *TableService) Update(ctx context.Context, id int64, in entity.UpdateTableInput) (*entity.Table, error){
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err // already *exception.Exception, just forward
	}

	existing.Number = in.Number
	existing.IsActive = in.IsActive

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	

	s.log.Info("table updated", "id", existing.ID, "number", existing.Number)
	return existing, nil
}

// Delete services
func (s *TableService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.log.Info("table deleted", "id", id)
	return nil
}

func (s *TableService) BulkDelete(ctx context.Context, ids []int64) ([]int64, error) {
	deleted, err := s.repo.BulkDelete(ctx, ids)
	if err != nil {
		return nil, err
	}

	s.log.Info("tables bulk deleted", "count", len(deleted), "ids", deleted)
	return deleted, nil
}