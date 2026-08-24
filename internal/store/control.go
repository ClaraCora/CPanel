package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) AuthenticateAgent(ctx context.Context, tokenHash []byte) (domain.AgentMachine, error) {
	var machine domain.AgentMachine
	err := s.pool.QueryRow(ctx, `
		SELECT m.id,m.name,m.status,m.kernel_type,c.id,m.capabilities
		FROM machine_credentials c JOIN machines m ON m.id=c.machine_id
		WHERE c.token_hash=$1 AND c.revoked_at IS NULL
		  AND (c.expires_at IS NULL OR c.expires_at > now())
		  AND m.status NOT IN ('disabled','archived')`, tokenHash,
	).Scan(&machine.ID, &machine.Name, &machine.Status, &machine.KernelType, &machine.TokenID, &machine.Capabilities)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentMachine{}, ErrNotFound
	}
	if err != nil {
		return domain.AgentMachine{}, err
	}
	_, _ = s.pool.Exec(ctx, "UPDATE machine_credentials SET last_used_at=now() WHERE id=$1", machine.TokenID)
	return machine, nil
}

func (s *Store) AuthenticateSharedAgent(ctx context.Context, machineID string, tokenHash []byte) (domain.AgentMachine, error) {
	var machine domain.AgentMachine
	err := s.pool.QueryRow(ctx, `
		SELECT m.id,m.name,m.status,m.kernel_type,'shared',m.capabilities
		FROM agent_shared_credentials c CROSS JOIN machines m
		WHERE c.token_hash=$1 AND m.id=$2 AND m.status NOT IN ('disabled','archived')`,
		tokenHash, machineID,
	).Scan(&machine.ID, &machine.Name, &machine.Status, &machine.KernelType, &machine.TokenID, &machine.Capabilities)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentMachine{}, ErrNotFound
	}
	return machine, err
}

func (s *Store) AgentNodes(ctx context.Context, machineID string) ([]domain.AgentNode, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT agent_id,protocol,name FROM nodes
		WHERE machine_id=$1 AND status='published' ORDER BY agent_id`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AgentNode, 0)
	for rows.Next() {
		var item domain.AgentNode
		if err := rows.Scan(&item.AgentID, &item.Type, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AgentNodeSpec(ctx context.Context, machineID string, agentNodeID int64) (domain.AgentNodeSpec, error) {
	var item domain.AgentNodeSpec
	var defaultPolicyID, adminPolicyID, memberPolicyID *string
	err := s.pool.QueryRow(ctx, `SELECT n.agent_id,n.current_revision,n.protocol,n.listen_ip,n.server_port,n.kernel_type,n.config,
		       n.route_policy_id,n.admin_route_policy_id,n.member_route_policy_id
		FROM nodes n
		WHERE n.machine_id=$1 AND n.agent_id=$2 AND n.status='published'`, machineID, agentNodeID).Scan(
		&item.NodeID, &item.Revision, &item.Protocol, &item.ListenIP, &item.ServerPort, &item.KernelType, &item.Settings,
		&defaultPolicyID, &adminPolicyID, &memberPolicyID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentNodeSpec{}, ErrNotFound
	}
	if err != nil {
		return domain.AgentNodeSpec{}, err
	}
	if defaultPolicyID == nil && adminPolicyID == nil && memberPolicyID == nil {
		return item, nil
	}
	allOutbounds, err := s.agentRouteOutbounds(ctx, item.KernelType)
	if err != nil {
		return domain.AgentNodeSpec{}, err
	}
	profiles := make(map[string]map[string]any, 3)
	defaultProfile, err := s.agentRouteProfile(ctx, defaultPolicyID, allOutbounds)
	if err != nil {
		return domain.AgentNodeSpec{}, err
	}
	if defaultProfile == nil {
		defaultProfile = emptyAgentRouteProfile()
	}
	profiles["default"] = defaultProfile
	for scope, policyID := range map[string]*string{"admin": adminPolicyID, "member": memberPolicyID} {
		profile := defaultProfile
		if policyID != nil && (defaultPolicyID == nil || *policyID != *defaultPolicyID) {
			profile, err = s.agentRouteProfile(ctx, policyID, allOutbounds)
			if err != nil {
				return domain.AgentNodeSpec{}, err
			}
			if len(profile) == 0 {
				profile = defaultProfile
			}
		}
		profiles[scope] = profile
	}
	item.Settings, err = mergeAgentRoutingProfiles(item.Settings, profiles)
	if err != nil {
		return domain.AgentNodeSpec{}, err
	}
	return item, err
}

func (s *Store) agentRouteOutbounds(ctx context.Context, kernelType string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `SELECT jsonb_strip_nulls(jsonb_build_object(
		'tag',o.tag,'protocol',o.protocol,'settings',o.settings,'proxy_tag',NULLIF(o.proxy_tag,'')))
		FROM outbounds o WHERE o.status='active' AND o.protocol NOT IN ('direct','block') AND $1=ANY(o.kernel_support)
		ORDER BY o.created_at`, kernelType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := make([]map[string]any, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item map[string]any
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		all = append(all, item)
	}
	return all, rows.Err()
}

func (s *Store) agentRouteProfile(ctx context.Context, policyID *string, allOutbounds []map[string]any) (map[string]any, error) {
	profile := emptyAgentRouteProfile()
	if policyID == nil || strings.TrimSpace(*policyID) == "" {
		return profile, nil
	}
	var published bool
	var defaultTag string
	var rawRules []byte
	err := s.pool.QueryRow(ctx, `SELECT r.status='published',COALESCE(r.default_outbound_tag,''),COALESCE((SELECT rr.rules FROM route_policy_revisions rr WHERE rr.route_policy_id=r.id ORDER BY rr.revision DESC LIMIT 1),'[]'::jsonb) FROM route_policies r WHERE r.id=$1`, *policyID).Scan(&published, &defaultTag, &rawRules)
	if errors.Is(err, pgx.ErrNoRows) || !published {
		return nil, nil
	}
	var rules []domain.RoutePolicyRule
	if err := json.Unmarshal(rawRules, &rules); err != nil {
		return nil, fmt.Errorf("decode route policy rules: %w", err)
	}
	profile["custom_route_rules"] = routeRulesForAgent(rules)
	profile["default_outbound_tag"] = effectiveAgentDefaultOutboundTag(defaultTag)
	profile["custom_outbounds"] = selectRouteOutbounds(rules, allOutbounds, defaultTag)
	return profile, nil
}

func emptyAgentRouteProfile() map[string]any {
	return map[string]any{"custom_route_rules": []domain.RoutePolicyRule{}, "default_outbound_tag": "direct", "custom_outbounds": []map[string]any{}}
}

func effectiveAgentDefaultOutboundTag(value string) string {
	if tag := strings.TrimSpace(value); tag != "" {
		return tag
	}
	return "direct"
}

func mergeAgentRoutingProfiles(settings json.RawMessage, profiles map[string]map[string]any) (json.RawMessage, error) {
	config := make(map[string]any)
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &config); err != nil {
			return nil, fmt.Errorf("decode node settings: %w", err)
		}
	}
	encodedProfiles := make(map[string]any, len(profiles))
	for scope, profile := range profiles {
		encodedProfiles[scope] = profile
	}
	config["custom_route_profiles"] = encodedProfiles
	if defaultProfile, ok := profiles["default"]; ok {
		config["custom_route_rules"] = defaultProfile["custom_route_rules"]
		config["default_outbound_tag"] = defaultProfile["default_outbound_tag"]
		config["custom_outbounds"] = mergeRouteOutbounds(config["custom_outbounds"], routeProfileOutbounds(defaultProfile))
	}
	for scope, profile := range profiles {
		if scope == "default" {
			continue
		}
		config["custom_outbounds"] = mergeRouteOutbounds(config["custom_outbounds"], routeProfileOutbounds(profile))
	}
	return json.Marshal(config)
}

func routeProfileOutbounds(profile map[string]any) []map[string]any {
	if value, ok := profile["custom_outbounds"].([]map[string]any); ok {
		return value
	}
	return nil
}

func mergeAgentRoutingSettings(settings, rawRules, rawOutbounds json.RawMessage, defaultOutboundTag string, policyPublished bool) (json.RawMessage, error) {
	config := make(map[string]any)
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &config); err != nil {
			return nil, fmt.Errorf("decode node settings: %w", err)
		}
	}
	rules := make([]domain.RoutePolicyRule, 0)
	defaultOutboundTag = strings.TrimSpace(defaultOutboundTag)
	if policyPublished && len(rawRules) > 0 {
		if err := json.Unmarshal(rawRules, &rules); err != nil {
			return nil, fmt.Errorf("decode route policy rules: %w", err)
		}
	}
	if !policyPublished {
		defaultOutboundTag = ""
	}
	defaultOutboundTag = effectiveAgentDefaultOutboundTag(defaultOutboundTag)
	config["custom_route_rules"] = routeRulesForAgent(rules)
	config["default_outbound_tag"] = defaultOutboundTag

	allOutbounds := make([]map[string]any, 0)
	if len(rawOutbounds) > 0 {
		if err := json.Unmarshal(rawOutbounds, &allOutbounds); err != nil {
			return nil, fmt.Errorf("decode route outbounds: %w", err)
		}
	}
	selected := selectRouteOutbounds(rules, allOutbounds, defaultOutboundTag)
	if len(selected) > 0 {
		config["custom_outbounds"] = mergeRouteOutbounds(config["custom_outbounds"], selected)
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode node routing settings: %w", err)
	}
	return encoded, nil
}

func routeRulesForAgent(rules []domain.RoutePolicyRule) []domain.RoutePolicyRule {
	result := make([]domain.RoutePolicyRule, len(rules))
	copy(result, rules)
	for index := range result {
		result[index].Match.Domains = append([]string{}, rules[index].Match.Domains...)
		result[index].Match.GeoIPs = append([]string{}, rules[index].Match.GeoIPs...)
		for _, expression := range rules[index].Match.DomainRegexes {
			expression = strings.TrimSpace(expression)
			if expression == "" {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(expression), "regexp:") {
				expression = "regexp:" + expression
			}
			result[index].Match.Domains = append(result[index].Match.Domains, expression)
		}
		result[index].Match.DomainRegexes = nil
	}
	return result
}

func selectRouteOutbounds(rules []domain.RoutePolicyRule, all []map[string]any, defaultOutboundTag string) []map[string]any {
	byTag := make(map[string]map[string]any, len(all))
	for _, outbound := range all {
		tag, _ := outbound["tag"].(string)
		if tag != "" {
			byTag[strings.ToLower(strings.TrimSpace(tag))] = outbound
		}
	}
	wanted := make(map[string]struct{})
	var include func(string, map[string]bool)
	include = func(tag string, visiting map[string]bool) {
		key := strings.ToLower(strings.TrimSpace(tag))
		if key == "" {
			return
		}
		if _, exists := wanted[key]; exists {
			return
		}
		if visiting[key] {
			return
		}
		outbound, exists := byTag[key]
		if !exists {
			return
		}
		visiting[key] = true
		wanted[key] = struct{}{}
		if proxyTag, _ := outbound["proxy_tag"].(string); proxyTag != "" {
			include(proxyTag, visiting)
		}
		delete(visiting, key)
	}
	for _, rule := range rules {
		if !rule.Disabled && rule.Action.Type == "route" {
			include(rule.Action.Target, map[string]bool{})
		}
	}
	include(defaultOutboundTag, map[string]bool{})
	selected := make([]map[string]any, 0, len(wanted))
	for _, outbound := range all {
		tag, _ := outbound["tag"].(string)
		if _, exists := wanted[strings.ToLower(strings.TrimSpace(tag))]; exists {
			selected = append(selected, outbound)
		}
	}
	return selected
}

func mergeRouteOutbounds(existing any, managed []map[string]any) []map[string]any {
	managedTags := make(map[string]struct{}, len(managed))
	for _, outbound := range managed {
		tag, _ := outbound["tag"].(string)
		managedTags[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	result := make([]map[string]any, 0, len(managed))
	if current, ok := existing.([]any); ok {
		for _, value := range current {
			outbound, ok := value.(map[string]any)
			if !ok {
				continue
			}
			tag, _ := outbound["tag"].(string)
			if _, replaced := managedTags[strings.ToLower(strings.TrimSpace(tag))]; !replaced {
				result = append(result, outbound)
			}
		}
	}
	return append(result, managed...)
}

func (s *Store) AgentNodeUsers(ctx context.Context, machineID string, agentNodeID int64) ([]domain.AgentUser, error) {
	rows, err := s.pool.Query(ctx, runtimeEligibleUsersCTE+`
		SELECT u.agent_id,u.uuid,
		       u.speed_limit_mbps,
		       u.device_limit,
		       CASE WHEN u.role='admin' THEN 'admin' ELSE 'member' END
		FROM nodes n
		JOIN access_group_nodes gn ON gn.node_id=n.id
		JOIN runtime_eligible_users u ON u.access_group_id=gn.access_group_id
		WHERE n.machine_id=$1 AND n.agent_id=$2 AND n.status='published'
		ORDER BY u.agent_id`, machineID, agentNodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AgentUser, 0)
	for rows.Next() {
		var item domain.AgentUser
		if err := rows.Scan(&item.ID, &item.UUID, &item.SpeedLimit, &item.DeviceLimit, &item.RouteScope); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AgentChanges(ctx context.Context, machineID string, after int64, limit int) ([]domain.ControlChange, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.cursor,n.agent_id,c.event_type,c.revision,c.payload,c.created_at
		FROM control_changes c LEFT JOIN nodes n ON n.id=c.node_id
		WHERE c.machine_id=$1 AND c.cursor>$2 ORDER BY c.cursor LIMIT $3`, machineID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ControlChange, 0)
	for rows.Next() {
		var item domain.ControlChange
		if err := rows.Scan(&item.Cursor, &item.NodeID, &item.EventType, &item.Revision, &item.Payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) LatestAgentCursor(ctx context.Context, machineID string) (int64, error) {
	var cursor int64
	err := s.pool.QueryRow(ctx, "SELECT COALESCE(max(cursor),0) FROM control_changes WHERE machine_id=$1", machineID).Scan(&cursor)
	return cursor, err
}

func (s *Store) RecordMachineHeartbeat(ctx context.Context, machine domain.AgentMachine, version, kernel string, capabilities, metrics json.RawMessage) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if len(capabilities) == 0 {
		capabilities = json.RawMessage(`{}`)
	}
	if len(metrics) == 0 {
		metrics = json.RawMessage(`{}`)
	}
	command, err := tx.Exec(ctx, `UPDATE machines SET status='online',agent_version=$2,kernel_type=$3,
		capabilities=$4,last_heartbeat_at=now(),updated_at=now()
		WHERE id=$1 AND status NOT IN ('disabled','archived')`,
		machine.ID, version, kernel, capabilities)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	// Older Agents can execute the detached installer but do not know how to
	// send the upgrade lifecycle ACK/result messages. A heartbeat that reports
	// the requested target version is authoritative evidence that such an
	// upgrade completed. This also repairs a task that timed out locally while
	// the Agent was restarting.
	var upgradeTaskID, upgradeStatus, upgradeTargetVersion string
	err = tx.QueryRow(ctx, `SELECT COALESCE(agent_upgrade_task_id,''),
		COALESCE(agent_upgrade_status,''),COALESCE(agent_upgrade_target_version,'')
		FROM machines WHERE id=$1`, machine.ID).Scan(&upgradeTaskID, &upgradeStatus, &upgradeTargetVersion)
	if err != nil {
		return err
	}
	if upgradeTaskID != "" && shouldCompleteAgentUpgradeFromHeartbeat(upgradeStatus, version, upgradeTargetVersion) {
		if _, err := tx.Exec(ctx, `UPDATE machines SET
			agent_upgrade_status='succeeded',
			agent_upgrade_acknowledged_at=COALESCE(agent_upgrade_acknowledged_at,now()),
			agent_upgrade_completed_at=COALESCE(agent_upgrade_completed_at,now()),
			agent_upgrade_failed_at=NULL,agent_upgrade_error='',updated_at=now()
			WHERE id=$1 AND agent_upgrade_task_id=$2
			  AND agent_upgrade_status IN ('dispatched','acknowledged','timed_out')`, machine.ID, upgradeTaskID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO machine_metrics(machine_id,sampled_at,metrics) VALUES($1,$2,$3)
		ON CONFLICT(machine_id) DO UPDATE SET sampled_at=EXCLUDED.sampled_at,metrics=EXCLUDED.metrics`,
		machine.ID, time.Now().UTC(), metrics)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ClaimMachineAgentUpgrade(ctx context.Context, machineID string) (domain.AgentCommand, bool, error) {
	var command domain.AgentCommand
	err := s.pool.QueryRow(ctx, `UPDATE machines SET agent_upgrade_dispatched_at=now(),
		agent_upgrade_status='dispatched',updated_at=now()
		WHERE id=$1 AND agent_upgrade_task_id IS NOT NULL AND agent_upgrade_dispatched_at IS NULL
			AND agent_upgrade_status IN ('queued','') AND status NOT IN ('disabled','archived')
		RETURNING agent_upgrade_task_id,COALESCE(agent_upgrade_target_version,'')`, machineID).Scan(&command.ID, &command.TargetVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentCommand{}, false, nil
	}
	if err != nil {
		return domain.AgentCommand{}, false, err
	}
	command.Type = "agent.upgrade"
	return command, true, nil
}

// ReportMachineAgentUpgrade records an explicit Agent acknowledgement/result.
// A normal heartbeat never completes or clears an upgrade task by itself.
func (s *Store) ReportMachineAgentUpgrade(ctx context.Context, machineID, taskID, status, version, message string) (domain.AgentUpgradeTask, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "acknowledged" && status != "succeeded" && status != "failed" && status != "timed_out" {
		return domain.AgentUpgradeTask{}, ErrConflict
	}
	version = strings.TrimSpace(version)
	if len(message) > 2000 {
		message = message[:2000]
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.AgentUpgradeTask{}, err
	}
	defer tx.Rollback(ctx)
	var targetVersion, currentStatus, currentVersion string
	var task domain.AgentUpgradeTask
	if err := tx.QueryRow(ctx, `SELECT COALESCE(agent_upgrade_target_version,''),COALESCE(agent_upgrade_status,''),agent_version,
		id,agent_upgrade_task_id,COALESCE(agent_upgrade_requested_at,created_at),agent_upgrade_dispatched_at,
		agent_upgrade_acknowledged_at,agent_upgrade_completed_at,agent_upgrade_failed_at,COALESCE(agent_upgrade_error,'')
		FROM machines WHERE id=$1 AND agent_upgrade_task_id=$2 FOR UPDATE`, machineID, taskID).
		Scan(&targetVersion, &currentStatus, &currentVersion, &task.MachineID, &task.ID, &task.RequestedAt,
			&task.DispatchedAt, &task.AcknowledgedAt, &task.CompletedAt, &task.FailedAt, &task.Error); errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentUpgradeTask{}, ErrNotFound
	} else if err != nil {
		return domain.AgentUpgradeTask{}, err
	}
	task.TargetVersion = targetVersion
	task.Status = currentStatus
	if currentStatus == status {
		// Retries are expected when an ACK/response was lost. Returning the stored
		// state without writing keeps terminal timestamps and errors immutable.
		return task, nil
	}
	if !validAgentUpgradeTransition(currentStatus, status) {
		return domain.AgentUpgradeTask{}, ErrConflict
	}
	if status == "succeeded" {
		if version == "" {
			version = currentVersion
		}
		if !agentTargetVersionMatches(version, targetVersion) {
			return domain.AgentUpgradeTask{}, ErrConflict
		}
	}
	err = tx.QueryRow(ctx, `UPDATE machines SET
		agent_upgrade_status=$3,
		agent_upgrade_acknowledged_at=CASE WHEN $3='acknowledged' AND agent_upgrade_acknowledged_at IS NULL THEN now() ELSE agent_upgrade_acknowledged_at END,
		agent_upgrade_completed_at=CASE WHEN $3='succeeded' THEN now() ELSE agent_upgrade_completed_at END,
		agent_upgrade_failed_at=CASE WHEN $3 IN ('failed','timed_out') THEN now() ELSE agent_upgrade_failed_at END,
		agent_upgrade_error=CASE WHEN $3 IN ('failed','timed_out') THEN $5 ELSE '' END,
		agent_version=CASE WHEN $3='succeeded' AND NULLIF($4,'') IS NOT NULL THEN $4 ELSE agent_version END,
		updated_at=now()
		WHERE id=$1 AND agent_upgrade_task_id=$2
		RETURNING id,agent_upgrade_task_id,COALESCE(agent_upgrade_target_version,''),COALESCE(agent_upgrade_status,''),
			agent_upgrade_requested_at,agent_upgrade_dispatched_at,agent_upgrade_acknowledged_at,
			agent_upgrade_completed_at,agent_upgrade_failed_at,COALESCE(agent_upgrade_error,'')`,
		machineID, taskID, status, version, message).Scan(&task.MachineID, &task.ID, &task.TargetVersion, &task.Status,
		&task.RequestedAt, &task.DispatchedAt, &task.AcknowledgedAt, &task.CompletedAt, &task.FailedAt, &task.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentUpgradeTask{}, ErrNotFound
	}
	if err != nil {
		return domain.AgentUpgradeTask{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AgentUpgradeTask{}, err
	}
	return task, nil
}

func validAgentUpgradeTransition(current, next string) bool {
	switch current {
	case "dispatched":
		// ACK and result are independent requests. If the ACK is lost, a
		// version-validated success result can still complete the task.
		return next == "acknowledged" || next == "succeeded" || next == "failed"
	case "acknowledged":
		return next == "succeeded" || next == "failed" || next == "timed_out"
	case "timed_out":
		// A late, authenticated result is stronger evidence than the local
		// timeout and may complete the task when the target matches.
		return next == "succeeded"
	default:
		return false
	}
}

func shouldCompleteAgentUpgradeFromHeartbeat(status, currentVersion, targetVersion string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "dispatched", "acknowledged", "timed_out":
		return agentTargetVersionMatches(currentVersion, targetVersion)
	default:
		return false
	}
}

func agentTargetVersionMatches(current, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "latest" || target == "corade-latest" {
		return strings.TrimSpace(current) != ""
	}
	return sameAgentVersion(current, target)
}

func sameAgentVersion(left, right string) bool {
	left = normalizeAgentVersionForComparison(left)
	right = normalizeAgentVersionForComparison(right)
	return left != "" && left == right
}

func normalizeAgentVersionForComparison(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "corade-")
	value = strings.TrimPrefix(value, "v")
	// Release artifacts include the source hash as build metadata. The upgrade
	// target is the immutable semantic base version, so compare that base while
	// still rejecting unrelated versions.
	if index := strings.IndexByte(value, '+'); index >= 0 {
		value = value[:index]
	}
	return value
}

func (s *Store) RecordTelemetryBatch(ctx context.Context, machineID, idempotencyKey string, payload json.RawMessage) (bool, error) {
	batch, err := decodeTelemetryBatch(payload)
	if err != nil {
		return false, err
	}
	trafficLocation := loadTrafficLocation(s.SettingString(ctx, "site", "timezone", "Asia/Shanghai"))
	deviceOfflineThreshold := s.machineOfflineThreshold(ctx)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO telemetry_batches(machine_id,idempotency_key)
		VALUES($1,$2) ON CONFLICT DO NOTHING`, machineID, idempotencyKey)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	// Device snapshots are point-in-time reports. Expire entries only after the
	// idempotency insert succeeds so replaying an already accepted batch has no
	// database side effects.
	if _, err := tx.Exec(ctx, `UPDATE user_devices SET online=false
		WHERE online AND last_seen_at < now() - ($1::bigint * interval '1 second')`, int64(deviceOfflineThreshold/time.Second)); err != nil {
		return false, err
	}
	if err := lockTelemetryDeviceUsers(ctx, tx, batch); err != nil {
		return false, err
	}
	for _, event := range batch.Events {
		switch event.Type {
		case "node.telemetry":
			if err := recordNodeTelemetry(ctx, tx, machineID, event, trafficLocation); err != nil {
				return false, err
			}
		case "node.devices":
			if err := recordNodeDevices(ctx, tx, machineID, event, deviceOfflineThreshold); err != nil {
				return false, err
			}
		}
	}
	return true, tx.Commit(ctx)
}

func lockTelemetryDeviceUsers(ctx context.Context, tx pgx.Tx, batch telemetryBatch) error {
	agentIDs := make(map[int64]bool)
	for _, event := range batch.Events {
		if event.Type != "node.devices" {
			continue
		}
		var data nodeDevicesData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("decode node devices: %w", err)
		}
		for rawUserID := range data.Devices {
			agentID, err := strconv.ParseInt(rawUserID, 10, 64)
			if err == nil && agentID > 0 {
				agentIDs[agentID] = true
			}
		}
	}
	if len(agentIDs) == 0 {
		return nil
	}
	ordered := make([]int64, 0, len(agentIDs))
	for agentID := range agentIDs {
		ordered = append(ordered, agentID)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	rows, err := tx.Query(ctx, `SELECT id FROM users WHERE agent_id=ANY($1) ORDER BY agent_id FOR UPDATE`, ordered)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return err
		}
	}
	return rows.Err()
}

type telemetryBatch struct {
	Events []telemetryEvent `json:"events"`
}

type telemetryEvent struct {
	Type       string          `json:"type"`
	NodeID     int64           `json:"node_id"`
	OccurredAt string          `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

type nodeTelemetryData struct {
	Revision int64               `json:"revision"`
	Traffic  map[string][2]int64 `json:"traffic"`
}

type nodeDevicesData struct {
	Devices map[string][]string `json:"devices"`
}

type deviceUserReport struct {
	AgentID   int64
	Addresses []string
}

func decodeTelemetryBatch(payload json.RawMessage) (telemetryBatch, error) {
	var batch telemetryBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		return telemetryBatch{}, fmt.Errorf("decode telemetry batch: %w", err)
	}
	if batch.Events == nil {
		return telemetryBatch{}, fmt.Errorf("telemetry events are required")
	}
	for index, event := range batch.Events {
		if event.Type == "" {
			return telemetryBatch{}, fmt.Errorf("telemetry event %d type is required", index)
		}
		if event.NodeID < 0 {
			return telemetryBatch{}, fmt.Errorf("telemetry event %d node_id is invalid", index)
		}
	}
	return batch, nil
}

const (
	telemetryMaximumFutureSkew = 5 * time.Minute
	telemetryMaximumPastAge    = 24 * time.Hour
)

func telemetryTime(value string) time.Time {
	return boundedTelemetryTime(value, time.Now().UTC())
}

// boundedTelemetryTime prevents an Agent with a broken clock (or a forged
// event timestamp) from moving node presence into the future or attributing a
// retry to an arbitrarily old traffic day. Invalid and out-of-window values
// are treated as received now.
func boundedTelemetryTime(value string, now time.Time) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		parsed = parsed.UTC()
		if !parsed.After(now.Add(telemetryMaximumFutureSkew)) && !parsed.Before(now.Add(-telemetryMaximumPastAge)) {
			return parsed
		}
	}
	return now.UTC()
}

func loadTrafficLocation(name string) *time.Location {
	if location, err := time.LoadLocation(strings.TrimSpace(name)); err == nil {
		return location
	}
	if location, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return location
	}
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

func telemetryDay(sampledAt time.Time, location *time.Location) string {
	if location == nil {
		location = loadTrafficLocation("")
	}
	return sampledAt.In(location).Format(time.DateOnly)
}

func recordNodeTelemetry(ctx context.Context, tx pgx.Tx, machineID string, event telemetryEvent, trafficLocation *time.Location) error {
	if event.NodeID <= 0 {
		return fmt.Errorf("node telemetry node_id is required")
	}
	var data nodeTelemetryData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("decode node telemetry: %w", err)
	}
	sampledAt := boundedTelemetryTime(event.OccurredAt, time.Now().UTC())
	var nodeID string
	err := tx.QueryRow(ctx, `
		UPDATE nodes SET applied_revision=CASE WHEN $3 > 0 THEN LEAST(current_revision,GREATEST(applied_revision,$3)) ELSE applied_revision END,
		       last_report_at=GREATEST(COALESCE(LEAST(last_report_at,now()),'epoch'::timestamptz),LEAST($4,now())),updated_at=now()
		WHERE machine_id=$1 AND agent_id=$2 AND status='published'
		RETURNING id`, machineID, event.NodeID, data.Revision, sampledAt).Scan(&nodeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO node_metrics(node_id,sampled_at,metrics) VALUES($1,LEAST($2,now()),$3)
		ON CONFLICT(node_id) DO UPDATE SET sampled_at=LEAST(EXCLUDED.sampled_at,now()),metrics=EXCLUDED.metrics
		WHERE LEAST(EXCLUDED.sampled_at,now()) >= LEAST(node_metrics.sampled_at,now())`, nodeID, sampledAt, event.Data); err != nil {
		return err
	}
	for userAgentID, traffic := range data.Traffic {
		userID, err := strconv.ParseInt(userAgentID, 10, 64)
		if err != nil || userID <= 0 || traffic[0] < 0 || traffic[1] < 0 || (traffic[0] == 0 && traffic[1] == 0) {
			continue
		}
		_, err = tx.Exec(ctx, runtimeEligibleUsersCTE+`, entitled AS (
			  SELECT u.id AS user_id,n.id AS node_id
			FROM nodes n
			JOIN access_group_nodes gn ON gn.node_id=n.id
			JOIN runtime_eligible_users u ON u.agent_id=$3
			WHERE n.id=$1 AND n.machine_id=$2
			  AND gn.access_group_id=u.access_group_id
			), recorded AS (
			  INSERT INTO traffic_daily(day,user_id,node_id,upload_bytes,download_bytes)
			  SELECT $4::date,user_id,node_id,$5,$6 FROM entitled
			ON CONFLICT(day,user_id,node_id) DO UPDATE SET
			  upload_bytes=traffic_daily.upload_bytes+EXCLUDED.upload_bytes,
			  download_bytes=traffic_daily.download_bytes+EXCLUDED.download_bytes
			  RETURNING user_id
			)
			UPDATE users SET traffic_used_bytes=traffic_used_bytes+$5+$6
			WHERE id IN (SELECT user_id FROM recorded)`,
			nodeID, machineID, userID, telemetryDay(sampledAt, trafficLocation), traffic[0], traffic[1])
		if err != nil {
			return err
		}
	}
	return nil
}

func recordNodeDevices(ctx context.Context, tx pgx.Tx, machineID string, event telemetryEvent, offlineThreshold time.Duration) error {
	if event.NodeID <= 0 {
		return fmt.Errorf("node devices node_id is required")
	}
	var data nodeDevicesData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("decode node devices: %w", err)
	}
	sampledAt := boundedTelemetryTime(event.OccurredAt, time.Now().UTC())
	online := !sampledAt.Before(time.Now().UTC().Add(-offlineThreshold))
	for _, report := range normalizeDeviceReports(data.Devices) {
		var userID, nodeID, nodeName string
		err := tx.QueryRow(ctx, runtimeEligibleUsersCTE+`
			SELECT u.id,n.id,n.name
			FROM nodes n
			JOIN access_group_nodes gn ON gn.node_id=n.id
			JOIN runtime_eligible_users u ON u.agent_id=$3
			WHERE n.machine_id=$1 AND n.agent_id=$2 AND n.status='published'
			  AND gn.access_group_id=u.access_group_id`,
			machineID, event.NodeID, report.AgentID).Scan(&userID, &nodeID, &nodeName)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		for _, address := range report.Addresses {
			_, err = tx.Exec(ctx, `
				INSERT INTO user_devices(id,user_id,node_id,ip_address,first_seen_at,last_seen_at,online)
				VALUES($1,$2,$3,$4::inet,$5,$5,$6)
				ON CONFLICT(user_id,node_id,ip_address) DO UPDATE SET
					first_seen_at=LEAST(user_devices.first_seen_at,EXCLUDED.first_seen_at),
					last_seen_at=GREATEST(user_devices.last_seen_at,EXCLUDED.last_seen_at),
					online=CASE WHEN EXCLUDED.online THEN true ELSE user_devices.online END`,
				domain.MustID("dev"), userID, nodeID, address, sampledAt, online)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO user_access_ips(user_id,ip_address,first_seen_at,last_seen_at,last_node_id,last_node_name)
				VALUES($1,$2::inet,$3,$3,$4,$5)
				ON CONFLICT(user_id,ip_address) DO UPDATE SET
					first_seen_at=LEAST(user_access_ips.first_seen_at,EXCLUDED.first_seen_at),
					last_seen_at=GREATEST(user_access_ips.last_seen_at,EXCLUDED.last_seen_at),
					last_node_id=CASE WHEN EXCLUDED.last_seen_at >= user_access_ips.last_seen_at THEN EXCLUDED.last_node_id ELSE user_access_ips.last_node_id END,
					last_node_name=CASE WHEN EXCLUDED.last_seen_at >= user_access_ips.last_seen_at THEN EXCLUDED.last_node_name ELSE user_access_ips.last_node_name END`,
				userID, address, sampledAt, nodeID, nodeName)
			if err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM user_access_ips
			WHERE user_id=$1 AND ip_address IN (
				SELECT ip_address FROM user_access_ips WHERE user_id=$1
				ORDER BY last_seen_at DESC,ip_address DESC OFFSET 10
			)`, userID); err != nil {
			return err
		}
	}
	return nil
}

func normalizeDeviceReports(devices map[string][]string) []deviceUserReport {
	reports := make([]deviceUserReport, 0, len(devices))
	for rawUserID, rawAddresses := range devices {
		agentID, err := strconv.ParseInt(rawUserID, 10, 64)
		if err != nil || agentID <= 0 {
			continue
		}
		seen := make(map[string]bool, len(rawAddresses))
		addresses := make([]string, 0, len(rawAddresses))
		for _, rawAddress := range rawAddresses {
			address := net.ParseIP(strings.TrimSpace(rawAddress))
			if address == nil {
				continue
			}
			normalized := address.String()
			if seen[normalized] {
				continue
			}
			seen[normalized] = true
			addresses = append(addresses, normalized)
		}
		if len(addresses) == 0 {
			continue
		}
		sort.Strings(addresses)
		reports = append(reports, deviceUserReport{AgentID: agentID, Addresses: addresses})
	}
	sort.Slice(reports, func(left, right int) bool { return reports[left].AgentID < reports[right].AgentID })
	return reports
}
