package application

import (
	"context"
	"strings"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// SocialService handles bookmarks, likes and comments.
type SocialService struct{ store Store }

func NewSocialService(store Store) *SocialService { return &SocialService{store: store} }

func (s *SocialService) Bookmark(ctx context.Context, userID, seriesID int64) error {
	if _, err := s.store.GetSeries(ctx, seriesID); err != nil {
		return domain.ErrNotFound("Series")
	}
	return s.store.SetBookmark(ctx, userID, seriesID)
}

func (s *SocialService) Unbookmark(ctx context.Context, userID, seriesID int64) error {
	return s.store.RemoveBookmark(ctx, userID, seriesID)
}

func (s *SocialService) Bookmarks(ctx context.Context, userID int64) ([]domain.Series, error) {
	return s.store.ListBookmarks(ctx, userID)
}

func (s *SocialService) Like(ctx context.Context, userID, episodeID int64) error {
	if _, err := s.store.GetEpisode(ctx, episodeID); err != nil {
		return domain.ErrNotFound("Episode")
	}
	return s.store.SetLike(ctx, userID, episodeID)
}

func (s *SocialService) Unlike(ctx context.Context, userID, episodeID int64) error {
	return s.store.RemoveLike(ctx, userID, episodeID)
}

func (s *SocialService) Comments(ctx context.Context, episodeID, userID int64, page, limit int) ([]domain.Comment, int, error) {
	if _, err := s.store.GetEpisode(ctx, episodeID); err != nil {
		return nil, 0, domain.ErrNotFound("Episode")
	}
	return s.store.ListComments(ctx, episodeID, userID, page, limit)
}

// RateSeries creates or replaces the caller star rating (1-5) for a series.
func (s *SocialService) RateSeries(ctx context.Context, userID, seriesID int64, stars int) (domain.SeriesRatingSummary, error) {
	if stars < 1 || stars > 5 {
		return domain.SeriesRatingSummary{}, domain.ErrBadRequest("VALIDATION_ERROR", "Rating harus 1-5 bintang.")
	}
	if _, err := s.store.GetSeries(ctx, seriesID); err != nil {
		return domain.SeriesRatingSummary{}, domain.ErrNotFound("Series")
	}
	if err := s.store.SetSeriesRating(ctx, userID, seriesID, stars); err != nil {
		return domain.SeriesRatingSummary{}, err
	}
	return s.store.SeriesRatingSummary(ctx, seriesID, userID)
}

// SeriesRating returns the computed rating summary; user stars only for a logged-in caller.
func (s *SocialService) SeriesRating(ctx context.Context, seriesID, userID int64) (domain.SeriesRatingSummary, error) {
	if _, err := s.store.GetSeries(ctx, seriesID); err != nil {
		return domain.SeriesRatingSummary{}, domain.ErrNotFound("Series")
	}
	return s.store.SeriesRatingSummary(ctx, seriesID, userID)
}

// LikeComment likes a comment; returns the real count and the caller new state.
func (s *SocialService) LikeComment(ctx context.Context, userID, commentID int64) (domain.CommentLikeStatus, error) {
	if err := s.store.SetCommentLike(ctx, userID, commentID); err != nil {
		return domain.CommentLikeStatus{}, err
	}
	return s.store.CommentLikeStatus(ctx, commentID, userID)
}

// UnlikeComment removes the caller like on a comment.
func (s *SocialService) UnlikeComment(ctx context.Context, userID, commentID int64) (domain.CommentLikeStatus, error) {
	if err := s.store.RemoveCommentLike(ctx, userID, commentID); err != nil {
		return domain.CommentLikeStatus{}, err
	}
	return s.store.CommentLikeStatus(ctx, commentID, userID)
}

func (s *SocialService) AddComment(ctx context.Context, userID int64, c *domain.Comment) error {
	if strings.TrimSpace(c.Body) == "" {
		return domain.ErrBadRequest("VALIDATION_ERROR", "Isi komentar wajib diisi.")
	}
	if _, err := s.store.GetEpisode(ctx, c.EpisodeID); err != nil {
		return domain.ErrNotFound("Episode")
	}
	c.UserID = userID
	return s.store.CreateComment(ctx, c)
}
