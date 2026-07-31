package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/support/cache"
	"coffeeshop/internal/support/exception"
	"coffeeshop/internal/support/qrtoken"
	"coffeeshop/internal/support/token"
)

// TableSession adalah hasil penukaran QR token.
type TableSession struct {
	Table     *entity.Table
	Token     string
	ExpiresAt time.Time
}

type TableService struct {
	repo  *repository.TableRepository
	cache *cache.TableCache
	rev   *cache.RevocationStore
	tm    *token.Manager
	log   *slog.Logger
}

func NewTableService(
	repo *repository.TableRepository,
	cache *cache.TableCache,
	rev *cache.RevocationStore,
	tm *token.Manager,
	log *slog.Logger,
) *TableService {
	return &TableService{repo: repo, cache: cache, rev: rev, tm: tm, log: log}
}

// ============================================================================
// Jalur pelanggan
// ============================================================================

// CreateSession menukar QR token meja dengan session token berumur pendek.
//
// Ini jalur panas — setiap pelanggan yang scan lewat sini. Urutannya:
// cache → negative cache → database, lalu hasilnya (positif maupun negatif)
// di-cache. Epoch pencabutan meja ikut ditanam di token.
func (s *TableService) CreateSession(ctx context.Context, qrToken string) (*TableSession, error) {
	table, err := s.resolveQRToken(ctx, qrToken)
	if err != nil {
		return nil, err
	}

	epoch, err := s.rev.CurrentEpoch(ctx, token.TableScope(table.ID))
	if err != nil {
		// Redis bermasalah: terbitkan dengan epoch 0. Token tetap sah,
		// tapi pencabutan massal sebelumnya jadi tidak terbawa.
		s.log.Warn("read table epoch failed, issuing with epoch 0", "id", table.ID, "err", err)
		epoch = 0
	}

	signed, expiresAt, err := s.tm.IssueTableSession(table.ID, table.Number, epoch)
	if err != nil {
		return nil, exception.Internal(err)
	}

	return &TableSession{Table: table, Token: signed, ExpiresAt: expiresAt}, nil
}

// resolveQRToken mencari meja aktif berdasarkan QR token.
//
// Token tidak dikenal dan meja nonaktif menghasilkan error yang identik,
// supaya tidak bisa dipakai memetakan token mana yang valid.
func (s *TableService) resolveQRToken(ctx context.Context, qrToken string) (*entity.Table, error) {
	invalid := exception.NotFound("TABLE_404", "invalid QR code")

	cached, err := s.cache.GetByToken(ctx, qrToken)
	switch {
	case errors.Is(err, cache.ErrNegativeHit):
		return nil, invalid
	case err != nil:
		s.log.Warn("cache lookup by token failed", "err", err)
	case cached != nil:
		return cached, nil
	}

	t, err := s.repo.FindActiveByQRToken(ctx, qrToken)
	if err != nil {
		var ex *exception.Exception
		if errors.As(err, &ex) && ex.Code == http.StatusNotFound {
			if cacheErr := s.cache.SetTokenMiss(ctx, qrToken); cacheErr != nil {
				s.log.Warn("cache set token miss failed", "err", cacheErr)
			}
			return nil, invalid
		}
		return nil, err
	}

	if err := s.cache.SetByToken(ctx, t); err != nil {
		s.log.Warn("cache set by token failed", "id", t.ID, "err", err)
	}
	return t, nil
}

// ============================================================================
// Pencabutan
// ============================================================================

// EndSessions mencabut SEMUA session yang sedang berjalan di satu meja.
//
// Dipakai saat kasir mengosongkan meja (pelanggan sudah pergi). Tanpa ini,
// orang yang sempat scan bisa terus memesan sampai 2 jam ke depan meski
// sudah tidak di kedai.
func (s *TableService) EndSessions(ctx context.Context, tableID int64) error {
	if _, err := s.repo.FindByID(ctx, tableID); err != nil {
		return err
	}

	epoch, err := s.rev.BumpEpoch(ctx, token.TableScope(tableID))
	if err != nil {
		return exception.Internal(fmt.Errorf("end table sessions: %w", err))
	}

	s.log.Info("table sessions revoked", "id", tableID, "epoch", epoch)
	return nil
}

// ============================================================================
// Jalur staff
// ============================================================================

func (s *TableService) List(ctx context.Context, in entity.ListTablesInput) (*entity.ListResult, error) {
	items, total, err := s.repo.List(ctx, in)
	if err != nil {
		return nil, err
	}
	return &entity.ListResult{
		Items:   items,
		Total:   total,
		Page:    in.Page,
		PerPage: in.PerPage,
	}, nil
}

func (s *TableService) GetByID(ctx context.Context, id int64) (*entity.Table, error) {
	cached, err := s.cache.GetByID(ctx, id)
	if err != nil {
		s.log.Warn("cache get by id failed", "id", id, "err", err)
	} else if cached != nil {
		return cached, nil
	}

	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.cache.SetByID(ctx, t); err != nil {
		s.log.Warn("cache set by id failed", "id", id, "err", err)
	}
	return t, nil
}

func (s *TableService) Create(ctx context.Context, in entity.CreateTableInput) (*entity.Table, error) {
	qr, err := qrtoken.GenerateQRToken()
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("generate qr token: %w", err))
	}

	t := &entity.Table{Number: in.Number, QRToken: qr, IsActive: in.IsActive}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}

	s.log.Info("table created", "id", t.ID, "number", t.Number)
	return t, nil
}

func (s *TableService) Update(ctx context.Context, id int64, in entity.UpdateTableInput) (*entity.Table, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	wasActive := existing.IsActive
	existing.Number = in.Number
	existing.IsActive = in.IsActive

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	s.invalidate(ctx, existing)

	// Menonaktifkan meja harus ikut memutus session yang sedang jalan,
	// kalau tidak pelanggan di meja itu masih bisa memesan.
	if wasActive && !existing.IsActive {
		if _, err := s.rev.BumpEpoch(ctx, token.TableScope(id)); err != nil {
			s.log.Warn("revoke sessions on deactivate failed", "id", id, "err", err)
		}
	}

	s.log.Info("table updated", "id", existing.ID, "number", existing.Number)
	return existing, nil
}

// RotateQRToken menerbitkan QR token baru dan memutus semua session lama.
//
// Ini prosedur pemulihan kalau stiker QR bocor (kefoto, tersebar di grup).
// Token lama langsung tidak berlaku, dan session yang terlanjur terbit
// ikut dicabut — kalau tidak, rotate-nya sia-sia selama 2 jam ke depan.
func (s *TableService) RotateQRToken(ctx context.Context, id int64) (*entity.Table, error) {
	old, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newToken, err := qrtoken.GenerateQRToken()
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("generate qr token: %w", err))
	}

	updated, err := s.repo.UpdateQRToken(ctx, id, newToken)
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx, old) // buang key token lama
	s.invalidate(ctx, updated)

	if _, err := s.rev.BumpEpoch(ctx, token.TableScope(id)); err != nil {
		s.log.Warn("revoke sessions on rotate failed", "id", id, "err", err)
	}

	s.log.Info("qr token rotated", "id", id, "number", updated.Number)
	return updated, nil
}

func (s *TableService) Delete(ctx context.Context, id int64) error {
	// Ambil dulu supaya QR token-nya diketahui dan key cache-nya bisa dibuang.
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.invalidate(ctx, existing)
	if _, err := s.rev.BumpEpoch(ctx, token.TableScope(id)); err != nil {
		s.log.Warn("revoke sessions on delete failed", "id", id, "err", err)
	}

	s.log.Info("table deleted", "id", id)
	return nil
}

func (s *TableService) invalidate(ctx context.Context, t *entity.Table) {
	if err := s.cache.Invalidate(ctx, t); err != nil {
		s.log.Warn("cache invalidate failed", "id", t.ID, "err", err)
	}
}