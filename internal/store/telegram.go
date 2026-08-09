package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type TelegramBotState struct {
	UpdateOffset  int64
	LastReportDay *time.Time
}

func (s *Store) PrepareTelegramBotState(ctx context.Context, fingerprint string) (TelegramBotState, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TelegramBotState{}, err
	}
	defer tx.Rollback(ctx)

	var storedFingerprint string
	var state TelegramBotState
	err = tx.QueryRow(ctx, `SELECT bot_fingerprint,update_offset,last_report_day
		FROM telegram_bot_state WHERE singleton=1 FOR UPDATE`).Scan(&storedFingerprint, &state.UpdateOffset, &state.LastReportDay)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO telegram_bot_state(singleton,bot_fingerprint) VALUES(1,$1)`, fingerprint)
		storedFingerprint = fingerprint
	}
	if err != nil {
		return TelegramBotState{}, err
	}
	if storedFingerprint != fingerprint {
		state = TelegramBotState{}
		if _, err := tx.Exec(ctx, `UPDATE telegram_bot_state SET bot_fingerprint=$1,update_offset=0,
			last_report_day=NULL,updated_at=now() WHERE singleton=1`, fingerprint); err != nil {
			return TelegramBotState{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramBotState{}, err
	}
	return state, nil
}

func (s *Store) SetTelegramUpdateOffset(ctx context.Context, fingerprint string, offset int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE telegram_bot_state SET update_offset=GREATEST(update_offset,$2),updated_at=now()
		WHERE singleton=1 AND bot_fingerprint=$1`, fingerprint, offset)
	return err
}

func (s *Store) MarkTelegramReportSent(ctx context.Context, fingerprint, day string) error {
	_, err := s.pool.Exec(ctx, `UPDATE telegram_bot_state SET last_report_day=$2::date,updated_at=now()
		WHERE singleton=1 AND bot_fingerprint=$1`, fingerprint, day)
	return err
}
