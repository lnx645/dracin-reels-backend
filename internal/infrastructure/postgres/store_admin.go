package postgres

import (
	"context"
	"strings"

	"github.com/lnx645/dracin-reels-backend/internal/application"
)

// --- Admin panel (admin role only) ---

const adminUserColumns = `u.id, u.name, u.email, u.role, u.is_active,
	COALESCE(w.balance, 0),
	EXISTS(SELECT 1 FROM vip_subscriptions v
		WHERE v.user_id = u.id AND v.status = 'active' AND v.ends_at > now()),
	u.created_at`

func scanAdminUser(rows interface{ Scan(...interface{}) error }) (application.AdminUserView, error) {
	var v application.AdminUserView
	err := rows.Scan(&v.ID, &v.Name, &v.Email, &v.Role, &v.IsActive,
		&v.CoinBalance, &v.VIPActive, &v.CreatedAt)
	return v, err
}

func (s *Store) ListAdminUsers(ctx context.Context, search string, page, limit int) ([]application.AdminUserView, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	search = strings.TrimSpace(search)
	where := ""
	args := []interface{}{}
	if search != "" {
		where = `WHERE u.name ILIKE '%' || $1 || '%' OR u.email ILIKE '%' || $1 || '%'`
		args = append(args, search)
	}
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users u `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+adminUserColumns+`
		 FROM users u LEFT JOIN coin_wallets w ON w.user_id = u.id `+where+`
		 ORDER BY u.created_at DESC, u.id DESC LIMIT $`+itoa(len(args)+1)+` OFFSET $`+itoa(len(args)+2),
		append(args, limit, (page-1)*limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []application.AdminUserView{}
	for rows.Next() {
		v, err := scanAdminUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (s *Store) GetAdminUser(ctx context.Context, id int64) (*application.AdminUserView, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+adminUserColumns+`
		 FROM users u LEFT JOIN coin_wallets w ON w.user_id = u.id
		 WHERE u.id = $1`, id)
	v, err := scanAdminUser(row)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UpdateAdminUser changes only role and/or is_active. Column names come
// from fixed branches only, never from request input, so the dynamic
// SET clause cannot be injected. Password hashes are never touched.
func (s *Store) UpdateAdminUser(ctx context.Context, id int64, role *string, isActive *bool) (*application.AdminUserView, error) {
	sets := []string{"updated_at = now()"}
	args := []interface{}{id}
	if role != nil {
		sets = append(sets, "role = $"+itoa(len(args)+1))
		args = append(args, *role)
	}
	if isActive != nil {
		sets = append(sets, "is_active = $"+itoa(len(args)+1))
		args = append(args, *isActive)
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id = $1`, args...); err != nil {
		return nil, err
	}
	return s.GetAdminUser(ctx, id)
}

// effectiveVIPStatus is the status the admin panel shows: a stored
// "active" row past its ends_at counts as expired.
const effectiveVIPStatus = `CASE WHEN v.status = 'active' AND v.ends_at > now() THEN 'active' ELSE 'expired' END`

func (s *Store) ListAdminVIPSubscriptions(ctx context.Context, status string, page, limit int) ([]application.AdminVIPView, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	inner := `SELECT v.id, v.user_id, u.email, v.plan, v.starts_at, v.ends_at,
		` + effectiveVIPStatus + ` AS eff_status, v.auto_renew, v.created_at
		FROM vip_subscriptions v JOIN users u ON u.id = v.user_id`
	where := ""
	args := []interface{}{}
	if status != "" {
		where = ` WHERE eff_status = $1`
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (`+inner+`) s`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT * FROM (`+inner+`) s`+where+`
		 ORDER BY created_at DESC, id DESC LIMIT $`+itoa(len(args)+1)+` OFFSET $`+itoa(len(args)+2),
		append(args, limit, (page-1)*limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []application.AdminVIPView{}
	for rows.Next() {
		var v application.AdminVIPView
		if err := rows.Scan(&v.ID, &v.UserID, &v.UserEmail, &v.Plan, &v.StartsAt,
			&v.EndsAt, &v.Status, &v.AutoRenew, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (s *Store) ListAdminTransactions(ctx context.Context, userID int64, txType string, page, limit int) ([]application.AdminTransactionView, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	where := ""
	args := []interface{}{}
	if userID > 0 {
		where += ` AND t.user_id = $` + itoa(len(args)+1)
		args = append(args, userID)
	}
	if txType != "" {
		where += ` AND t.type = $` + itoa(len(args)+1)
		args = append(args, txType)
	}
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM coin_transactions t WHERE 1=1`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.user_id, u.email, t.type, t.amount, t.balance_after, t.reference_id, t.created_at
		 FROM coin_transactions t JOIN users u ON u.id = t.user_id
		 WHERE 1=1`+where+`
		 ORDER BY t.created_at DESC, t.id DESC LIMIT $`+itoa(len(args)+1)+` OFFSET $`+itoa(len(args)+2),
		append(args, limit, (page-1)*limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []application.AdminTransactionView{}
	for rows.Next() {
		var v application.AdminTransactionView
		var ref sqlNullString
		if err := rows.Scan(&v.ID, &v.UserID, &v.UserEmail, &v.Type, &v.Amount,
			&v.BalanceAfter, &ref, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		v.ReferenceID = ref.ptr()
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (s *Store) AdminStats(ctx context.Context) (*application.AdminStats, error) {
	st := &application.AdminStats{
		LatestSeries:   []application.AdminSeriesView{},
		LatestEpisodes: []application.AdminEpisodeView{},
	}
	counts := []struct {
		dst *int
		q   string
	}{
		{&st.SeriesCount, `SELECT COUNT(*) FROM series`},
		{&st.EpisodeCount, `SELECT COUNT(*) FROM episodes`},
		{&st.UserCount, `SELECT COUNT(*) FROM users`},
		{&st.ActiveVIPCount, `SELECT COUNT(*) FROM vip_subscriptions WHERE status = 'active' AND ends_at > now()`},
		{&st.TransactionCount, `SELECT COUNT(*) FROM coin_transactions`},
		{&st.TransactionsToday, `SELECT COUNT(*) FROM coin_transactions WHERE created_at >= CURRENT_DATE`},
	}
	for _, c := range counts {
		if err := s.db.QueryRowContext(ctx, c.q).Scan(c.dst); err != nil {
			return nil, err
		}
	}
	srows, err := s.db.QueryContext(ctx,
		`SELECT id, title, status, created_at FROM series ORDER BY created_at DESC, id DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	for srows.Next() {
		var v application.AdminSeriesView
		if err := srows.Scan(&v.ID, &v.Title, &v.Status, &v.CreatedAt); err != nil {
			srows.Close()
			return nil, err
		}
		st.LatestSeries = append(st.LatestSeries, v)
	}
	if err := srows.Err(); err != nil {
		srows.Close()
		return nil, err
	}
	srows.Close()
	erows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.series_id, s.title, e.number, e.title, e.status, e.created_at
		 FROM episodes e JOIN series s ON s.id = e.series_id
		 ORDER BY e.created_at DESC, e.id DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer erows.Close()
	for erows.Next() {
		var v application.AdminEpisodeView
		if err := erows.Scan(&v.ID, &v.SeriesID, &v.SeriesTitle, &v.Number,
			&v.Title, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		st.LatestEpisodes = append(st.LatestEpisodes, v)
	}
	return st, erows.Err()
}
