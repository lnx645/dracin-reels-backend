package postgres

import "database/sql"

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func strPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
