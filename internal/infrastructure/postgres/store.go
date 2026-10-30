package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// Store implements application.Store on top of PostgreSQL.
type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// --- Users ---

func (s *Store) CreateUser(ctx context.Context, u *domain.User) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO users (name, email, password_hash, avatar_url, role)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
		u.Name, u.Email, u.PasswordHash, nullString(u.AvatarURL), u.Role,
	).Scan(&u.ID, &u.CreatedAt)
	// NOTE: no wallet row is created here; GetWallet creates a zero wallet
	// on demand, so registration never fails on wallet setup.
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	var avatar sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, avatar_url, role, created_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &avatar, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.AvatarURL = strPtr(avatar)
	return u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	u := &domain.User{}
	var avatar sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, avatar_url, role, created_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &avatar, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.AvatarURL = strPtr(avatar)
	return u, nil
}

// --- Genres ---

func (s *Store) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, slug FROM genres ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Genre{}
	for rows.Next() {
		var g domain.Genre
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) CreateGenre(ctx context.Context, g *domain.Genre) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO genres (name, slug) VALUES ($1,$2) RETURNING id`, g.Name, g.Slug,
	).Scan(&g.ID)
}

func (s *Store) UpdateGenre(ctx context.Context, g *domain.Genre) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE genres SET name=$1, slug=$2 WHERE id=$3`, g.Name, g.Slug, g.ID)
	return err
}

func (s *Store) DeleteGenre(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM genres WHERE id=$1`, id)
	return err
}

// --- Series ---

// total_episode is always computed as COUNT of published episodes,
// never read from the stored series.total_episode column (which goes stale
// whenever episodes are added/removed). rating has no rating/review
// mechanism behind it, so the stored value is returned as-is (0 = no rating
// data) and never fabricated.
func scanSeries(rows *sql.Rows) (domain.Series, error) {
	var s domain.Series
	var cover sql.NullString
	err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.Tagline, &cover, &s.Status,
		&s.TotalEpisode, &s.FreeEpisodeCount, &s.Rating, &s.CreatedAt, &s.UpdatedAt)
	s.CoverURL = strPtr(cover)
	return s, err
}

func (s *Store) attachGenres(ctx context.Context, series *domain.Series) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.slug FROM genres g
		 JOIN series_genres sg ON sg.genre_id = g.id
		 WHERE sg.series_id = $1 ORDER BY g.name`, series.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	series.Genres = []domain.Genre{}
	for rows.Next() {
		var g domain.Genre
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug); err != nil {
			return err
		}
		series.Genres = append(series.Genres, g)
	}
	return rows.Err()
}

func (s *Store) ListSeries(ctx context.Context, genreSlug string, genreID int64, status string, page, limit int) ([]domain.Series, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var conds []string
	var args []interface{}
	if genreID > 0 {
		args = append(args, genreID)
		conds = append(conds, `EXISTS (SELECT 1 FROM series_genres sg WHERE sg.series_id = series.id AND sg.genre_id = $`+itoa(len(args))+`)`)
	}
	if genreSlug != "" {
		args = append(args, genreSlug)
		conds = append(conds, `EXISTS (SELECT 1 FROM series_genres sg JOIN genres g ON g.id = sg.genre_id WHERE sg.series_id = series.id AND (g.slug = $`+itoa(len(args))+` OR LOWER(g.name) = LOWER($`+itoa(len(args))+`)))`)
	}
	if status != "" {
		args = append(args, status)
		conds = append(conds, `series.status = $`+itoa(len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM series `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, tagline, cover_url, status,
		 (SELECT COUNT(*) FROM episodes e WHERE e.series_id = series.id AND e.status='published'),
		 free_episode_count, rating, created_at, updated_at FROM series `+where+
			` ORDER BY created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.Series{}
	for rows.Next() {
		ser, err := scanSeries(rows)
		if err != nil {
			return nil, 0, err
		}
		if err := s.attachGenres(ctx, &ser); err != nil {
			return nil, 0, err
		}
		out = append(out, ser)
	}
	return out, total, rows.Err()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func (s *Store) GetSeries(ctx context.Context, id int64) (*domain.Series, error) {
	ser := &domain.Series{}
	var cover sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, tagline, cover_url, status,
		 (SELECT COUNT(*) FROM episodes e WHERE e.series_id = series.id AND e.status='published'),
		 free_episode_count, rating, created_at, updated_at FROM series WHERE id=$1`, id,
	).Scan(&ser.ID, &ser.Title, &ser.Description, &ser.Tagline, &cover, &ser.Status,
		&ser.TotalEpisode, &ser.FreeEpisodeCount, &ser.Rating, &ser.CreatedAt, &ser.UpdatedAt)
	if err != nil {
		return nil, err
	}
	ser.CoverURL = strPtr(cover)
	if err := s.attachGenres(ctx, ser); err != nil {
		return nil, err
	}
	return ser, nil
}

func (s *Store) CreateSeries(ctx context.Context, ser *domain.Series, genreIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx,
		`INSERT INTO series (title, description, tagline, cover_url, status, total_episode, free_episode_count, rating)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, created_at, updated_at`,
		ser.Title, ser.Description, ser.Tagline, nullString(ser.CoverURL), ser.Status,
		ser.TotalEpisode, ser.FreeEpisodeCount, ser.Rating,
	).Scan(&ser.ID, &ser.CreatedAt, &ser.UpdatedAt)
	if err != nil {
		return err
	}
	for _, gid := range genreIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO series_genres (series_id, genre_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, ser.ID, gid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateSeries(ctx context.Context, ser *domain.Series, genreIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE series SET title=$1, description=$2, tagline=$3, cover_url=$4, status=$5,
		 total_episode=$6, free_episode_count=$7, rating=$8, updated_at=now() WHERE id=$9`,
		ser.Title, ser.Description, ser.Tagline, nullString(ser.CoverURL), ser.Status,
		ser.TotalEpisode, ser.FreeEpisodeCount, ser.Rating, ser.ID); err != nil {
		return err
	}
	if genreIDs != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM series_genres WHERE series_id=$1`, ser.ID); err != nil {
			return err
		}
		for _, gid := range genreIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO series_genres (series_id, genre_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, ser.ID, gid); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteSeries(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM series WHERE id=$1`, id)
	return err
}

// --- Episodes ---

func scanEpisode(rows *sql.Rows) (domain.Episode, error) {
	var e domain.Episode
	var sub, thumb sql.NullString
	err := rows.Scan(&e.ID, &e.SeriesID, &e.Number, &e.Title, &e.Description,
		&e.VideoURL, &sub, &e.DurationSeconds, &e.CoinPrice, &e.Status, &thumb,
		&e.CreatedAt, &e.UpdatedAt)
	e.SubtitleURL = strPtr(sub)
	e.ThumbnailURL = strPtr(thumb)
	return e, err
}

const episodeCols = `id, series_id, number, title, description, video_url,
	subtitle_url, duration_seconds, coin_price, status, thumbnail_url, created_at, updated_at`

func (s *Store) ListEpisodesBySeries(ctx context.Context, seriesID int64, onlyPublished bool) ([]domain.Episode, error) {
	q := `SELECT ` + episodeCols + ` FROM episodes WHERE series_id=$1`
	args := []interface{}{seriesID}
	if onlyPublished {
		q += ` AND status='published'`
	}
	q += ` ORDER BY number ASC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Episode{}
	for rows.Next() {
		ep, err := scanEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ep)
	}
	return out, rows.Err()
}

func (s *Store) GetEpisode(ctx context.Context, id int64) (*domain.Episode, error) {
	ep := &domain.Episode{}
	var sub, thumb sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT `+episodeCols+` FROM episodes WHERE id=$1`, id,
	).Scan(&ep.ID, &ep.SeriesID, &ep.Number, &ep.Title, &ep.Description,
		&ep.VideoURL, &sub, &ep.DurationSeconds, &ep.CoinPrice, &ep.Status, &thumb,
		&ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		return nil, err
	}
	ep.SubtitleURL = strPtr(sub)
	ep.ThumbnailURL = strPtr(thumb)
	return ep, nil
}

func (s *Store) CreateEpisode(ctx context.Context, e *domain.Episode) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO episodes (series_id, number, title, description, video_url,
		 subtitle_url, duration_seconds, coin_price, status, thumbnail_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id, created_at, updated_at`,
		e.SeriesID, e.Number, e.Title, e.Description, e.VideoURL,
		nullString(e.SubtitleURL), e.DurationSeconds, e.CoinPrice, e.Status, nullString(e.ThumbnailURL),
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (s *Store) UpdateEpisode(ctx context.Context, e *domain.Episode) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE episodes SET series_id=$1, number=$2, title=$3, description=$4,
		 video_url=$5, subtitle_url=$6, duration_seconds=$7, coin_price=$8,
		 status=$9, thumbnail_url=$10, updated_at=now() WHERE id=$11`,
		e.SeriesID, e.Number, e.Title, e.Description, e.VideoURL,
		nullString(e.SubtitleURL), e.DurationSeconds, e.CoinPrice, e.Status,
		nullString(e.ThumbnailURL), e.ID)
	return err
}

func (s *Store) DeleteEpisode(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM episodes WHERE id=$1`, id)
	return err
}
