package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type assignedPlan struct {
	defaultValidDays int
	resetStrategy    string
}

// Holding a shared plan lock until the account mutation commits serializes new
// assignments with plan disabling/archiving. Existing bindings remain usable.
func assignedPlanTx(ctx context.Context, tx pgx.Tx, planID, existingPlanID *string) (assignedPlan, error) {
	if planID == nil || strings.TrimSpace(*planID) == "" {
		return assignedPlan{}, nil
	}
	var plan assignedPlan
	var status string
	err := tx.QueryRow(ctx, `SELECT status,default_valid_days,reset_strategy FROM plans WHERE id=$1 FOR SHARE`, *planID).
		Scan(&status, &plan.defaultValidDays, &plan.resetStrategy)
	if errors.Is(err, pgx.ErrNoRows) {
		return assignedPlan{}, ErrPlanUnavailable
	}
	if err != nil {
		return assignedPlan{}, err
	}
	existing := existingPlanID != nil && *existingPlanID == *planID
	if status == "archived" || (status != "active" && !existing) {
		return assignedPlan{}, ErrPlanUnavailable
	}
	return plan, nil
}

func userExpiry(mode string, requested *time.Time, defaultDays int, now time.Time, location *time.Location) (*time.Time, error) {
	switch mode {
	case "default":
		if defaultDays == 0 {
			return nil, nil
		}
		expires := now.In(location).AddDate(0, 0, defaultDays)
		return &expires, nil
	case "permanent":
		return nil, nil
	case "custom":
		if requested == nil || requested.IsZero() {
			return nil, ErrUserExpiryInvalid
		}
		return requested, nil
	default:
		return nil, ErrUserExpiryInvalid
	}
}

func updateUserExpiry(mode *string, requested *string, firstAssignment bool, defaultDays int, now time.Time, location *time.Location) (*string, error) {
	if mode == nil && requested == nil && !firstAssignment {
		return nil, nil // Editing other fields or changing a plan preserves expiry.
	}
	effectiveMode := "default"
	var date *time.Time
	if requested != nil {
		if strings.TrimSpace(*requested) == "" {
			effectiveMode = "permanent"
		} else {
			parsed, err := time.Parse(time.RFC3339, *requested)
			if err != nil {
				return nil, ErrUserExpiryInvalid
			}
			date = &parsed
			effectiveMode = "custom"
		}
	}
	if mode != nil {
		effectiveMode = *mode
	}
	expires, err := userExpiry(effectiveMode, date, defaultDays, now, location)
	if err != nil {
		return nil, err
	}
	value := ""
	if expires != nil {
		value = expires.Format(time.RFC3339Nano)
	}
	return &value, nil
}

func notifyUserPublishedNodesTx(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
		SELECT n.machine_id,n.id,'node.members.replace',n.current_revision,jsonb_build_object('node_id',n.agent_id)
		FROM nodes n JOIN access_group_nodes gn ON gn.node_id=n.id
		JOIN users u ON u.id=$1 LEFT JOIN plans p ON p.id=u.plan_id
		WHERE n.status='published' AND gn.access_group_id=COALESCE(u.access_group_override_id,p.access_group_id)`, userID)
	return err
}

func notifyAllPublishedNodesTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
		SELECT machine_id,id,'node.members.replace',current_revision,jsonb_build_object('node_id',agent_id)
		FROM nodes WHERE status='published'`)
	return err
}
