package application

import (
	"context"
	"time"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// AdminUserView is one row of the admin user list: account data plus the
// live coin balance and effective VIP state. Password hashes are never
// included.
type AdminUserView struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	IsActive    bool      `json:"is_active"`
	CoinBalance int       `json:"coin_balance"`
	VIPActive   bool      `json:"vip_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminVIPView is one VIP membership row with the owner's email. Status
// is the EFFECTIVE status: a stored "active" row past its ends_at counts
// as expired.
type AdminVIPView struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Plan      string    `json:"plan"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Status    string    `json:"status"`
	AutoRenew bool      `json:"auto_renew"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminTransactionView is one coin transaction with the owner's email.
type AdminTransactionView struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	UserEmail    string    `json:"user_email"`
	Type         string    `json:"type"`
	Amount       int       `json:"amount"`
	BalanceAfter int       `json:"balance_after"`
	ReferenceID  *string   `json:"reference_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// AdminSeriesView is the latest-series card row of the admin dashboard.
type AdminSeriesView struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminEpisodeView is the latest-episode row of the admin dashboard.
type AdminEpisodeView struct {
	ID          int64     `json:"id"`
	SeriesID    int64     `json:"series_id"`
	SeriesTitle string    `json:"series_title"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminStats is the admin dashboard summary. Every number is a live
// COUNT from the database; nothing is estimated or fabricated.
// TransactionsToday uses the database server's current date.
type AdminStats struct {
	SeriesCount       int                `json:"series_count"`
	EpisodeCount      int                `json:"episode_count"`
	UserCount         int                `json:"user_count"`
	ActiveVIPCount    int                `json:"active_vip_count"`
	TransactionCount  int                `json:"transaction_count"`
	TransactionsToday int                `json:"transactions_today"`
	LatestSeries      []AdminSeriesView  `json:"latest_series"`
	LatestEpisodes    []AdminEpisodeView `json:"latest_episodes"`
}

// AdminService implements the admin-panel usecases (dashboard, user and
// member management, full transaction list). All callers must already
// have passed the requireAdmin guard.
type AdminService struct {
	store Store
}

func NewAdminService(store Store) *AdminService {
	return &AdminService{store: store}
}

func (s *AdminService) Stats(ctx context.Context) (*AdminStats, error) {
	return s.store.AdminStats(ctx)
}

func (s *AdminService) ListUsers(ctx context.Context, search string, page, limit int) ([]AdminUserView, int, error) {
	return s.store.ListAdminUsers(ctx, search, page, limit)
}

func (s *AdminService) GetUser(ctx context.Context, id int64) (*AdminUserView, error) {
	// Raw error on purpose: a missing row surfaces as sql.ErrNoRows,
	// which the HTTP layer maps to 404, while a real DB failure stays
	// a 500 instead of a misleading "not found".
	return s.store.GetAdminUser(ctx, id)
}

// UpdateUser changes only role and/or active status. Password hashes are
// never touched here, and users are never hard-deleted (deactivation is
// the soft off-switch). A nil field means "leave unchanged".
func (s *AdminService) UpdateUser(ctx context.Context, id int64, role *string, isActive *bool) (*AdminUserView, error) {
	if role == nil && isActive == nil {
		return nil, domain.ErrBadRequest("VALIDATION_ERROR", "Tidak ada field yang diubah (role atau is_active wajib diisi salah satu).")
	}
	if role != nil && *role != domain.RoleViewer && *role != domain.RoleAdmin {
		return nil, domain.ErrBadRequest("VALIDATION_ERROR", "Role harus viewer atau admin.")
	}
	return s.store.UpdateAdminUser(ctx, id, role, isActive)
}

func (s *AdminService) ListVIPSubscriptions(ctx context.Context, status string, page, limit int) ([]AdminVIPView, int, error) {
	if status != "" && status != domain.VIPActive && status != domain.VIPExpired {
		return nil, 0, domain.ErrBadRequest("VALIDATION_ERROR", "Status harus active atau expired.")
	}
	return s.store.ListAdminVIPSubscriptions(ctx, status, page, limit)
}

func (s *AdminService) ListTransactions(ctx context.Context, userID int64, txType string, page, limit int) ([]AdminTransactionView, int, error) {
	if txType != "" {
		switch txType {
		case domain.TxCheckin, domain.TxTask, domain.TxPurchase, domain.TxUnlock, domain.TxReward:
		default:
			return nil, 0, domain.ErrBadRequest("VALIDATION_ERROR", "Jenis transaksi tidak dikenal.")
		}
	}
	return s.store.ListAdminTransactions(ctx, userID, txType, page, limit)
}
