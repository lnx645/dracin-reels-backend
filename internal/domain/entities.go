package domain

import "time"

// Role values.
const (
	RoleViewer = "viewer"
	RoleAdmin  = "admin"
)

// Series status values.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// VIP plan values.
const (
	PlanWeekly  = "weekly"
	PlanMonthly = "monthly"
	PlanYearly  = "yearly"
)

// VIP status values.
const (
	VIPActive  = "active"
	VIPExpired = "expired"
)

// Coin transaction types.
const (
	TxCheckin  = "checkin"
	TxTask     = "task"
	TxPurchase = "purchase"
	TxUnlock   = "unlock"
	TxReward   = "reward"
)

// Reward task types.
const (
	TaskCheckin      = "checkin"
	TaskWatchAd      = "watch_ad"
	TaskFinishSeries = "finish_series"
)

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	AvatarURL    *string   `json:"avatar_url"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	PasswordHash string    `json:"-"`
}

type Genre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Series struct {
	ID               int64   `json:"id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Tagline          string  `json:"tagline"`
	CoverURL         *string `json:"cover_url"`
	Status           string  `json:"status"`
	TotalEpisode     int     `json:"total_episode"`
	FreeEpisodeCount int     `json:"free_episode_count"`
	Rating           float64 `json:"rating"`
	// Computed from series_ratings (never fabricated): nil average + 0 count
	// means no user has rated yet. UserRating is set only for the caller.
	AverageRating *float64  `json:"average_rating"`
	RatingCount   int       `json:"rating_count"`
	UserRating    *int      `json:"user_rating,omitempty"`
	Genres        []Genre   `json:"genres,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Episode struct {
	ID              int64     `json:"id"`
	SeriesID        int64     `json:"series_id"`
	Number          int       `json:"number"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	VideoURL        string    `json:"video_url"`
	SubtitleURL     *string   `json:"subtitle_url"`
	DurationSeconds int       `json:"duration_seconds"`
	CoinPrice       int       `json:"coin_price"`
	Status          string    `json:"status"`
	ThumbnailURL    *string   `json:"thumbnail_url"`
	HLSURL          *string   `json:"hls_url"`
	TranscodeStatus string    `json:"transcode_status"`
	TranscodeError  *string   `json:"transcode_error,omitempty"`
	Locked          bool      `json:"locked"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CoinWallet struct {
	UserID    int64     `json:"user_id"`
	Balance   int       `json:"balance"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CoinTransaction struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Type         string    `json:"type"`
	Amount       int       `json:"amount"`
	BalanceAfter int       `json:"balance_after"`
	ReferenceID  *string   `json:"reference_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type CoinPackage struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	CoinAmount int       `json:"coin_amount"`
	PriceIDR   int       `json:"price_idr"`
	BonusCoins int       `json:"bonus_coins"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

type VIPSubscription struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Plan      string    `json:"plan"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Status    string    `json:"status"`
	AutoRenew bool      `json:"auto_renew"`
	CreatedAt time.Time `json:"created_at"`
}

type WatchProgress struct {
	UserID          int64     `json:"user_id"`
	EpisodeID       int64     `json:"episode_id"`
	PositionSeconds int       `json:"position_seconds"`
	Finished        bool      `json:"finished"`
	LastWatchedAt   time.Time `json:"last_watched_at"`
	Episode         *Episode  `json:"episode,omitempty"`
	Series          *Series   `json:"series,omitempty"`
}

type Comment struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	EpisodeID int64     `json:"episode_id"`
	Body      string    `json:"body"`
	UserName  string    `json:"user_name,omitempty"`
	LikeCount int       `json:"like_count"`
	LikedByMe bool      `json:"liked_by_me"`
	CreatedAt time.Time `json:"created_at"`
}

// SeriesRatingSummary is the computed rating state of one series.
// AverageRating is nil (not 0) when nobody has rated yet.
type SeriesRatingSummary struct {
	SeriesID      int64    `json:"series_id"`
	AverageRating *float64 `json:"average_rating"`
	RatingCount   int      `json:"rating_count"`
	UserStars     *int     `json:"user_stars,omitempty"`
}

// CommentLikeStatus is the real like state of one comment for one caller.
type CommentLikeStatus struct {
	CommentID int64 `json:"comment_id"`
	Liked     bool  `json:"liked"`
	LikeCount int   `json:"like_count"`
}

type RewardTask struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	CoinReward int    `json:"coin_reward"`
	DailyLimit int    `json:"daily_limit"`
}

// IsFree reports whether episode number n is inside the free range.
func (s *Series) IsFree(number int) bool {
	return number >= 1 && number <= s.FreeEpisodeCount
}

// VIPPlanDuration returns the duration of a VIP plan.
func VIPPlanDuration(plan string) (time.Duration, bool) {
	switch plan {
	case PlanWeekly:
		return 7 * 24 * time.Hour, true
	case PlanMonthly:
		return 30 * 24 * time.Hour, true
	case PlanYearly:
		return 365 * 24 * time.Hour, true
	default:
		return 0, false
	}
}
