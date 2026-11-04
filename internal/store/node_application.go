package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type nodeApplicationData struct {
	Revision int    `json:"revision"`
	Status   string `json:"status"`
	Stage    string `json:"stage"`
	Error    string `json:"error"`
}

// Application results share the authenticated, idempotent telemetry transport,
// but ordinary metric/traffic reports can never acknowledge a configuration.
func recordNodeApplication(ctx context.Context, tx pgx.Tx, machineID string, event telemetryEvent) error {
	var data nodeApplicationData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("decode node application result: %w", err)
	}
	if event.NodeID <= 0 || data.Revision <= 0 {
		return fmt.Errorf("node application result requires node and revision")
	}
	if data.Status != "received" && data.Status != "applied" && data.Status != "failed" {
		return fmt.Errorf("invalid node application status")
	}
	data.Stage = strings.TrimSpace(data.Stage)
	if len(data.Stage) > 64 {
		return fmt.Errorf("node application stage is too long")
	}
	message := strings.TrimSpace(data.Error)
	if len([]rune(message)) > 1000 {
		message = string([]rune(message)[:1000])
	}
	if data.Status != "failed" {
		message = ""
	}
	sampledAt := boundedTelemetryTime(event.OccurredAt, time.Now().UTC())
	// A delayed result for an older revision may confirm that older runtime,
	// but must not erase the failure/progress of a newer attempt. Replayed
	// received events likewise cannot downgrade a successful/failed attempt.
	_, err := tx.Exec(ctx, `UPDATE nodes SET
		received_revision=GREATEST(received_revision,$3),
		applied_revision=CASE WHEN $4='applied' THEN GREATEST(applied_revision,$3) ELSE applied_revision END,
		attempted_revision=GREATEST(attempted_revision,$3),
		apply_status=CASE WHEN $3>=attempted_revision AND
		  (($4='received' AND $3>applied_revision AND ($3>attempted_revision OR apply_status NOT IN ('failed','applied')))
		   OR ($4='failed' AND $3>applied_revision)
		   OR $4='applied') THEN $4 ELSE apply_status END,
		apply_stage=CASE WHEN $3>=attempted_revision AND
		  (($4='received' AND $3>applied_revision AND ($3>attempted_revision OR apply_status NOT IN ('failed','applied')))
		   OR ($4='failed' AND $3>applied_revision)
		   OR $4='applied') THEN $5 ELSE apply_stage END,
		last_error=CASE WHEN $3>=attempted_revision AND
		  (($4='failed' AND $3>applied_revision) OR $4='applied' OR ($4='received' AND $3>attempted_revision))
		  THEN $6 ELSE last_error END,
		last_report_at=GREATEST(COALESCE(LEAST(last_report_at,now()),'epoch'::timestamptz),LEAST($7,now())),
		updated_at=now()
		WHERE machine_id=$1 AND agent_id=$2 AND status='published' AND $3<=current_revision`,
		machineID, event.NodeID, data.Revision, data.Status, data.Stage, message, sampledAt)
	return err
}
