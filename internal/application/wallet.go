package application

import (
	"context"
	"fmt"
	"time"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// WalletService implements unlock, check-in, VIP and watch progress rules.
type WalletService struct{ store Store }

func NewWalletService(store Store) *WalletService { return &WalletService{store: store} }

func (s *WalletService) GetWallet(ctx context.Context, userID int64) (*domain.CoinWallet, error) {
	return s.store.GetWallet(ctx, userID)
}

func (s *WalletService) Transactions(ctx context.Context, userID int64, page, limit int) ([]domain.CoinTransaction, int, error) {
	return s.store.ListTransactions(ctx, userID, page, limit)
}

func (s *WalletService) CoinPackages(ctx context.Context, onlyActive bool) ([]domain.CoinPackage, error) {
	return s.store.ListCoinPackages(ctx, onlyActive)
}

func (s *WalletService) CreateCoinPackage(ctx context.Context, p *domain.CoinPackage) error {
	if p.Name == "" || p.CoinAmount <= 0 {
		return domain.ErrBadRequest("VALIDATION_ERROR", "Nama paket dan jumlah koin wajib diisi.")
	}
	return s.store.CreateCoinPackage(ctx, p)
}

func (s *WalletService) UpdateCoinPackage(ctx context.Context, p *domain.CoinPackage) error {
	if _, err := s.store.GetCoinPackage(ctx, p.ID); err != nil {
		return domain.ErrNotFound("Paket koin")
	}
	return s.store.UpdateCoinPackage(ctx, p)
}

func (s *WalletService) DeleteCoinPackage(ctx context.Context, id int64) error {
	return s.store.DeleteCoinPackage(ctx, id)
}

// UnlockEpisode implements rules 2, 5, 6, 8.
func (s *WalletService) UnlockEpisode(ctx context.Context, userID, episodeID int64) (*domain.CoinWallet, error) {
	ep, err := s.store.GetEpisode(ctx, episodeID)
	if err != nil {
		return nil, domain.ErrNotFound("Episode")
	}
	series, err := s.store.GetSeries(ctx, ep.SeriesID)
	if err != nil {
		return nil, domain.ErrNotFound("Series")
	}
	if series.IsFree(ep.Number) || ep.CoinPrice <= 0 {
		return nil, domain.ErrBadRequest("EPISODE_FREE", "Episode ini sudah gratis, tidak perlu dibuka dengan koin.")
	}
	if vip, err := s.store.ActiveVIP(ctx, userID); err == nil && vip != nil {
		return nil, domain.ErrBadRequest("VIP_ACTIVE", "VIP Anda masih aktif, semua episode sudah terbuka tanpa koin.")
	}
	if unlocked, err := s.store.IsEpisodeUnlocked(ctx, userID, episodeID); err == nil && unlocked {
		return nil, domain.ErrBadRequest("ALREADY_UNLOCKED", "Episode ini sudah pernah Anda buka.")
	}
	wallet, err := s.store.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}
	if wallet.Balance < ep.CoinPrice {
		return nil, domain.ErrBadRequest("INSUFFICIENT_BALANCE",
			fmt.Sprintf("Saldo koin tidak cukup. Harga episode %d koin, saldo Anda %d koin.", ep.CoinPrice, wallet.Balance))
	}
	return s.store.UnlockEpisode(ctx, userID, ep)
}

// Checkin implements rule 7: once per day.
func (s *WalletService) Checkin(ctx context.Context, userID int64) (*domain.CoinWallet, int, error) {
	task, err := s.store.GetCheckinTask(ctx)
	if err != nil {
		return nil, 0, domain.ErrBadRequest("TASK_NOT_FOUND", "Tugas check-in harian belum tersedia.")
	}
	today := time.Now()
	claimed, err := s.store.HasClaimedToday(ctx, userID, task.ID, today)
	if err != nil {
		return nil, 0, err
	}
	if claimed {
		return nil, 0, domain.ErrBadRequest("ALREADY_CHECKED_IN", "Anda sudah check-in hari ini, silakan kembali besok.")
	}
	wallet, err := s.store.ClaimCheckin(ctx, userID, task)
	if err != nil {
		// Race guard: unique constraint surfaces as already claimed.
		return nil, 0, domain.ErrBadRequest("ALREADY_CHECKED_IN", "Anda sudah check-in hari ini, silakan kembali besok.")
	}
	return wallet, task.CoinReward, nil
}

func (s *WalletService) ListRewardTasks(ctx context.Context) ([]domain.RewardTask, error) {
	return s.store.ListRewardTasks(ctx)
}

func (s *WalletService) VIPStatus(ctx context.Context, userID int64) (*domain.VIPSubscription, error) {
	vip, err := s.store.ActiveVIP(ctx, userID)
	if err != nil {
		return nil, err
	}
	return vip, nil
}

// ClaimVIP is a SAMPLE claim: no real payment exists in this phase.
// It only records a subscription so VIP flows can be tested end to end.
func (s *WalletService) ClaimVIP(ctx context.Context, userID int64, plan string) (*domain.VIPSubscription, error) {
	if _, ok := domain.VIPPlanDuration(plan); !ok {
		return nil, domain.ErrBadRequest("VALIDATION_ERROR", "Paket VIP tidak valid. Pilih weekly, monthly, atau yearly.")
	}
	return s.store.ClaimVIP(ctx, userID, plan)
}

// SaveWatchProgress implements rule 10: locked episodes are rejected.
func (s *WalletService) SaveWatchProgress(ctx context.Context, userID int64, wp *domain.WatchProgress) error {
	ep, err := s.store.GetEpisode(ctx, wp.EpisodeID)
	if err != nil {
		return domain.ErrNotFound("Episode")
	}
	series, err := s.store.GetSeries(ctx, ep.SeriesID)
	if err != nil {
		return domain.ErrNotFound("Series")
	}
	catalog := &CatalogService{store: s.store}
	locked, err := catalog.isLocked(ctx, series, ep, userID, false)
	if err != nil {
		return err
	}
	if locked {
		return domain.ErrBadRequest("EPISODE_LOCKED", "Episode ini masih terkunci. Buka dengan koin atau VIP aktif dulu.")
	}
	if wp.PositionSeconds < 0 {
		wp.PositionSeconds = 0
	}
	wp.UserID = userID
	return s.store.UpsertWatchProgress(ctx, wp)
}

func (s *WalletService) ContinueWatching(ctx context.Context, userID int64, limit int) ([]domain.WatchProgress, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.store.ContinueWatching(ctx, userID, limit)
}
