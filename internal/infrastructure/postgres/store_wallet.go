package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// --- Wallet, transactions, unlock, check-in ---

func (s *Store) GetWallet(ctx context.Context, userID int64) (*domain.CoinWallet, error) {
	w := &domain.CoinWallet{UserID: userID}
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, balance, updated_at FROM coin_wallets WHERE user_id=$1`, userID,
	).Scan(&w.UserID, &w.Balance, &w.UpdatedAt)
	if err != nil {
		// Create a zero wallet on demand (e.g. users registered before a wallet existed).
		if _, ierr := s.db.ExecContext(ctx,
			`INSERT INTO coin_wallets (user_id, balance) VALUES ($1, 0) ON CONFLICT (user_id) DO NOTHING`, userID); ierr != nil {
			return nil, ierr
		}
		if err2 := s.db.QueryRowContext(ctx,
			`SELECT user_id, balance, updated_at FROM coin_wallets WHERE user_id=$1`, userID,
		).Scan(&w.UserID, &w.Balance, &w.UpdatedAt); err2 != nil {
			return nil, err2
		}
	}
	return w, nil
}

func (s *Store) ListTransactions(ctx context.Context, userID int64, page, limit int) ([]domain.CoinTransaction, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM coin_transactions WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, type, amount, balance_after, reference_id, created_at
		 FROM coin_transactions WHERE user_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`,
		userID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.CoinTransaction{}
	for rows.Next() {
		var t domain.CoinTransaction
		var ref sqlNullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Type, &t.Amount, &t.BalanceAfter, &ref, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		t.ReferenceID = ref.ptr()
		out = append(out, t)
	}
	return out, total, rows.Err()
}

type sqlNullString struct {
	String string
	Valid  bool
}

func (n *sqlNullString) Scan(value interface{}) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	switch v := value.(type) {
	case string:
		n.String, n.Valid = v, true
	case []byte:
		n.String, n.Valid = string(v), true
	default:
		n.String, n.Valid = fmt.Sprint(v), true
	}
	return nil
}

func (n sqlNullString) ptr() *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func (s *Store) IsEpisodeUnlocked(ctx context.Context, userID, episodeID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM episode_unlocks WHERE user_id=$1 AND episode_id=$2)`,
		userID, episodeID).Scan(&exists)
	return exists, err
}

// UnlockEpisode runs balance deduction + transaction + unlock record in one tx.
// The service already validated price/balance; the tx re-checks balance.
func (s *Store) UnlockEpisode(ctx context.Context, userID int64, ep *domain.Episode) (*domain.CoinWallet, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var balance int
	if err := tx.QueryRowContext(ctx,
		`SELECT balance FROM coin_wallets WHERE user_id=$1 FOR UPDATE`, userID,
	).Scan(&balance); err != nil {
		// No wallet row yet: treat as zero balance.
		balance = 0
		if _, ierr := tx.ExecContext(ctx,
			`INSERT INTO coin_wallets (user_id, balance) VALUES ($1, 0) ON CONFLICT (user_id) DO NOTHING`, userID); ierr != nil {
			return nil, ierr
		}
	}
	if balance < ep.CoinPrice {
		return nil, domain.ErrBadRequest("INSUFFICIENT_BALANCE",
			fmt.Sprintf("Saldo koin tidak cukup. Harga episode %d koin, saldo Anda %d koin.", ep.CoinPrice, balance))
	}
	newBalance := balance - ep.CoinPrice
	if _, err := tx.ExecContext(ctx,
		`UPDATE coin_wallets SET balance=$1, updated_at=now() WHERE user_id=$2`, newBalance, userID); err != nil {
		return nil, err
	}
	ref := fmt.Sprintf("episode:%d", ep.ID)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO coin_transactions (user_id, type, amount, balance_after, reference_id)
		 VALUES ($1, 'unlock', $2, $3, $4)`, userID, -ep.CoinPrice, newBalance, ref); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO episode_unlocks (user_id, episode_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		userID, ep.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &domain.CoinWallet{UserID: userID, Balance: newBalance, UpdatedAt: time.Now()}, nil
}

func (s *Store) GetCheckinTask(ctx context.Context) (*domain.RewardTask, error) {
	t := &domain.RewardTask{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, type, coin_reward, daily_limit FROM reward_tasks WHERE type='checkin' LIMIT 1`,
	).Scan(&t.ID, &t.Name, &t.Type, &t.CoinReward, &t.DailyLimit)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) HasClaimedToday(ctx context.Context, userID, taskID int64, day time.Time) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM user_reward_claims WHERE user_id=$1 AND task_id=$2 AND claim_date=$3)`,
		userID, taskID, day.Format("2006-01-02")).Scan(&exists)
	return exists, err
}

// ClaimCheckin inserts the claim, credits the wallet and logs the transaction in one tx.
func (s *Store) ClaimCheckin(ctx context.Context, userID int64, task *domain.RewardTask) (*domain.CoinWallet, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	today := time.Now().Format("2006-01-02")
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_reward_claims (user_id, task_id, claim_date) VALUES ($1,$2,$3)`,
		userID, task.ID, today); err != nil {
		return nil, err
	}
	var balance int
	if err := tx.QueryRowContext(ctx,
		`SELECT balance FROM coin_wallets WHERE user_id=$1 FOR UPDATE`, userID,
	).Scan(&balance); err != nil {
		balance = 0
		if _, ierr := tx.ExecContext(ctx,
			`INSERT INTO coin_wallets (user_id, balance) VALUES ($1, 0) ON CONFLICT (user_id) DO NOTHING`, userID); ierr != nil {
			return nil, ierr
		}
	}
	newBalance := balance + task.CoinReward
	if _, err := tx.ExecContext(ctx,
		`UPDATE coin_wallets SET balance=$1, updated_at=now() WHERE user_id=$2`, newBalance, userID); err != nil {
		return nil, err
	}
	ref := fmt.Sprintf("task:checkin:%s", today)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO coin_transactions (user_id, type, amount, balance_after, reference_id)
		 VALUES ($1, 'checkin', $2, $3, $4)`, userID, task.CoinReward, newBalance, ref); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &domain.CoinWallet{UserID: userID, Balance: newBalance, UpdatedAt: time.Now()}, nil
}

func (s *Store) ListRewardTasks(ctx context.Context) ([]domain.RewardTask, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, coin_reward, daily_limit FROM reward_tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RewardTask{}
	for rows.Next() {
		var t domain.RewardTask
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.CoinReward, &t.DailyLimit); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// --- Coin packages ---

func (s *Store) ListCoinPackages(ctx context.Context, onlyActive bool) ([]domain.CoinPackage, error) {
	q := `SELECT id, name, coin_amount, price_idr, bonus_coins, active, created_at FROM coin_packages`
	if onlyActive {
		q += ` WHERE active = true`
	}
	q += ` ORDER BY price_idr ASC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.CoinPackage{}
	for rows.Next() {
		var p domain.CoinPackage
		if err := rows.Scan(&p.ID, &p.Name, &p.CoinAmount, &p.PriceIDR, &p.BonusCoins, &p.Active, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetCoinPackage(ctx context.Context, id int64) (*domain.CoinPackage, error) {
	p := &domain.CoinPackage{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, coin_amount, price_idr, bonus_coins, active, created_at FROM coin_packages WHERE id=$1`, id,
	).Scan(&p.ID, &p.Name, &p.CoinAmount, &p.PriceIDR, &p.BonusCoins, &p.Active, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) CreateCoinPackage(ctx context.Context, p *domain.CoinPackage) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO coin_packages (name, coin_amount, price_idr, bonus_coins, active)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
		p.Name, p.CoinAmount, p.PriceIDR, p.BonusCoins, p.Active,
	).Scan(&p.ID, &p.CreatedAt)
}

func (s *Store) UpdateCoinPackage(ctx context.Context, p *domain.CoinPackage) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE coin_packages SET name=$1, coin_amount=$2, price_idr=$3, bonus_coins=$4, active=$5 WHERE id=$6`,
		p.Name, p.CoinAmount, p.PriceIDR, p.BonusCoins, p.Active, p.ID)
	return err
}

func (s *Store) DeleteCoinPackage(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM coin_packages WHERE id=$1`, id)
	return err
}

// --- VIP ---

func (s *Store) ActiveVIP(ctx context.Context, userID int64) (*domain.VIPSubscription, error) {
	// Lazy expiry: an "active" row past ends_at no longer opens anything.
	if _, err := s.db.ExecContext(ctx,
		`UPDATE vip_subscriptions SET status='expired' WHERE user_id=$1 AND status='active' AND ends_at <= now()`, userID); err != nil {
		return nil, err
	}
	v := &domain.VIPSubscription{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, plan, starts_at, ends_at, status, auto_renew, created_at
		 FROM vip_subscriptions WHERE user_id=$1 AND status='active' AND ends_at > now()
		 ORDER BY ends_at DESC LIMIT 1`, userID,
	).Scan(&v.ID, &v.UserID, &v.Plan, &v.StartsAt, &v.EndsAt, &v.Status, &v.AutoRenew, &v.CreatedAt)
	if err != nil {
		return nil, nil // no active VIP is not an error
	}
	return v, nil
}

// ClaimVIP creates a sample subscription (no real payment) or extends an active one.
func (s *Store) ClaimVIP(ctx context.Context, userID int64, plan string) (*domain.VIPSubscription, error) {
	dur, ok := domain.VIPPlanDuration(plan)
	if !ok {
		return nil, domain.ErrBadRequest("VALIDATION_ERROR", "Paket VIP tidak valid.")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now()
	var existingID int64
	var existingEnds time.Time
	err = tx.QueryRowContext(ctx,
		`SELECT id, ends_at FROM vip_subscriptions WHERE user_id=$1 AND status='active' AND ends_at > now() ORDER BY ends_at DESC LIMIT 1`,
		userID).Scan(&existingID, &existingEnds)
	if err == nil {
		newEnds := existingEnds.Add(dur)
		if _, err := tx.ExecContext(ctx,
			`UPDATE vip_subscriptions SET ends_at=$1 WHERE id=$2`, newEnds, existingID); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		v := &domain.VIPSubscription{ID: existingID, UserID: userID, Plan: plan, StartsAt: now, EndsAt: newEnds, Status: domain.VIPActive}
		return v, nil
	}
	v := &domain.VIPSubscription{}
	err = tx.QueryRowContext(ctx,
		`INSERT INTO vip_subscriptions (user_id, plan, starts_at, ends_at, status, auto_renew)
		 VALUES ($1,$2,$3,$4,'active',false) RETURNING id, created_at`,
		userID, plan, now, now.Add(dur),
	).Scan(&v.ID, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	v.UserID, v.Plan, v.StartsAt, v.EndsAt, v.Status = userID, plan, now, now.Add(dur), domain.VIPActive
	return v, nil
}

// --- Watch progress ---

func (s *Store) UpsertWatchProgress(ctx context.Context, wp *domain.WatchProgress) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO watch_progress (user_id, episode_id, position_seconds, finished, last_watched_at)
		 VALUES ($1,$2,$3,$4,now())
		 ON CONFLICT (user_id, episode_id) DO UPDATE
		 SET position_seconds=EXCLUDED.position_seconds, finished=EXCLUDED.finished, last_watched_at=now()`,
		wp.UserID, wp.EpisodeID, wp.PositionSeconds, wp.Finished)
	return err
}

func (s *Store) ContinueWatching(ctx context.Context, userID int64, limit int) ([]domain.WatchProgress, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT wp.user_id, wp.episode_id, wp.position_seconds, wp.finished, wp.last_watched_at,
		 e.id, e.series_id, e.number, e.title, e.description, e.video_url, e.duration_seconds, e.coin_price, e.status,
		 sr.id, sr.title, sr.description, sr.cover_url, sr.status, sr.total_episode, sr.free_episode_count, sr.rating
		 FROM watch_progress wp
		 JOIN episodes e ON e.id = wp.episode_id
		 JOIN series sr ON sr.id = e.series_id
		 WHERE wp.user_id=$1 ORDER BY wp.last_watched_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.WatchProgress{}
	for rows.Next() {
		var wp domain.WatchProgress
		ep := &domain.Episode{}
		sr := &domain.Series{}
		var cover sqlNullString
		var epVideo string
		// subtitle/thumbnail omitted in this joined view (not needed for resume cards).
		if err := rows.Scan(&wp.UserID, &wp.EpisodeID, &wp.PositionSeconds, &wp.Finished, &wp.LastWatchedAt,
			&ep.ID, &ep.SeriesID, &ep.Number, &ep.Title, &ep.Description, &epVideo, &ep.DurationSeconds, &ep.CoinPrice, &ep.Status,
			&sr.ID, &sr.Title, &sr.Description, &cover, &sr.Status, &sr.TotalEpisode, &sr.FreeEpisodeCount, &sr.Rating); err != nil {
			return nil, err
		}
		ep.VideoURL = epVideo
		sr.CoverURL = cover.ptr()
		wp.Episode, wp.Series = ep, sr
		out = append(out, wp)
	}
	return out, rows.Err()
}

// --- Social ---

func (s *Store) SetBookmark(ctx context.Context, userID, seriesID int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bookmarks (user_id, series_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, seriesID)
	return err
}

func (s *Store) RemoveBookmark(ctx context.Context, userID, seriesID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM bookmarks WHERE user_id=$1 AND series_id=$2`, userID, seriesID)
	return err
}

func (s *Store) ListBookmarks(ctx context.Context, userID int64) ([]domain.Series, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sr.id, sr.title, sr.description, sr.cover_url, sr.status, sr.total_episode,
		 sr.free_episode_count, sr.rating, sr.created_at, sr.updated_at
		 FROM bookmarks b JOIN series sr ON sr.id = b.series_id
		 WHERE b.user_id=$1 ORDER BY b.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Series{}
	for rows.Next() {
		ser, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		if err := s.attachGenres(ctx, &ser); err != nil {
			return nil, err
		}
		out = append(out, ser)
	}
	return out, rows.Err()
}

func (s *Store) SetLike(ctx context.Context, userID, episodeID int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO likes (user_id, episode_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, episodeID)
	return err
}

func (s *Store) RemoveLike(ctx context.Context, userID, episodeID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM likes WHERE user_id=$1 AND episode_id=$2`, userID, episodeID)
	return err
}

func (s *Store) ListComments(ctx context.Context, episodeID int64, page, limit int) ([]domain.Comment, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM comments WHERE episode_id=$1`, episodeID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.user_id, c.episode_id, c.body, c.created_at, u.name
		 FROM comments c JOIN users u ON u.id = c.user_id
		 WHERE c.episode_id=$1 ORDER BY c.created_at ASC LIMIT $2 OFFSET $3`,
		episodeID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.UserID, &c.EpisodeID, &c.Body, &c.CreatedAt, &c.UserName); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (s *Store) CreateComment(ctx context.Context, c *domain.Comment) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO comments (user_id, episode_id, body) VALUES ($1,$2,$3) RETURNING id, created_at`,
		c.UserID, c.EpisodeID, c.Body,
	).Scan(&c.ID, &c.CreatedAt)
}
