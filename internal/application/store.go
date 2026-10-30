package application

import (
	"context"
	"time"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// Store is the persistence port used by all usecases. The postgres
// implementation lives in internal/infrastructure/postgres.
type Store interface {
	// Users & auth
	CreateUser(ctx context.Context, u *domain.User) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)

	// Genres
	ListGenres(ctx context.Context) ([]domain.Genre, error)
	CreateGenre(ctx context.Context, g *domain.Genre) error
	UpdateGenre(ctx context.Context, g *domain.Genre) error
	DeleteGenre(ctx context.Context, id int64) error

	// Series
	ListSeries(ctx context.Context, genreSlug string, genreID int64, status string, page, limit int) ([]domain.Series, int, error)
	GetSeries(ctx context.Context, id int64) (*domain.Series, error)
	CreateSeries(ctx context.Context, s *domain.Series, genreIDs []int64) error
	UpdateSeries(ctx context.Context, s *domain.Series, genreIDs []int64) error
	DeleteSeries(ctx context.Context, id int64) error

	// Episodes
	ListEpisodesBySeries(ctx context.Context, seriesID int64, onlyPublished bool) ([]domain.Episode, error)
	GetEpisode(ctx context.Context, id int64) (*domain.Episode, error)
	CreateEpisode(ctx context.Context, e *domain.Episode) error
	UpdateEpisode(ctx context.Context, e *domain.Episode) error
	DeleteEpisode(ctx context.Context, id int64) error

	// Wallet & unlock & check-in (each atomic inside the repository)
	GetWallet(ctx context.Context, userID int64) (*domain.CoinWallet, error)
	ListTransactions(ctx context.Context, userID int64, page, limit int) ([]domain.CoinTransaction, int, error)
	UnlockEpisode(ctx context.Context, userID int64, ep *domain.Episode) (*domain.CoinWallet, error)
	IsEpisodeUnlocked(ctx context.Context, userID, episodeID int64) (bool, error)
	ClaimCheckin(ctx context.Context, userID int64, task *domain.RewardTask) (*domain.CoinWallet, error)

	// Coin packages
	ListCoinPackages(ctx context.Context, onlyActive bool) ([]domain.CoinPackage, error)
	GetCoinPackage(ctx context.Context, id int64) (*domain.CoinPackage, error)
	CreateCoinPackage(ctx context.Context, p *domain.CoinPackage) error
	UpdateCoinPackage(ctx context.Context, p *domain.CoinPackage) error
	DeleteCoinPackage(ctx context.Context, id int64) error

	// VIP
	ActiveVIP(ctx context.Context, userID int64) (*domain.VIPSubscription, error)
	ClaimVIP(ctx context.Context, userID int64, plan string) (*domain.VIPSubscription, error)

	// Watch progress & continue watching
	UpsertWatchProgress(ctx context.Context, wp *domain.WatchProgress) error
	ContinueWatching(ctx context.Context, userID int64, limit int) ([]domain.WatchProgress, error)

	// Social
	SetBookmark(ctx context.Context, userID, seriesID int64) error
	RemoveBookmark(ctx context.Context, userID, seriesID int64) error
	ListBookmarks(ctx context.Context, userID int64) ([]domain.Series, error)
	SetLike(ctx context.Context, userID, episodeID int64) error
	RemoveLike(ctx context.Context, userID, episodeID int64) error
	ListComments(ctx context.Context, episodeID int64, page, limit int) ([]domain.Comment, int, error)
	CreateComment(ctx context.Context, c *domain.Comment) error

	// Reward tasks
	ListRewardTasks(ctx context.Context) ([]domain.RewardTask, error)
	GetCheckinTask(ctx context.Context) (*domain.RewardTask, error)
	HasClaimedToday(ctx context.Context, userID, taskID int64, day time.Time) (bool, error)
}
