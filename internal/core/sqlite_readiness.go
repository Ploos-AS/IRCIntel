package core

import (
	"context"
	"errors"
)

func (s *SQLiteStore) Ping(ctx context.Context) error {
	if s == nil || s.db == nil { return errors.New("sqlite store is not open") }
	return s.db.PingContext(ctx)
}
