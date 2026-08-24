package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// resourceQuerier is implemented by both pgx pools and transactions. All
// reference checks below accept a transaction so callers can validate and
// mutate a resource atomically.
type resourceQuerier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type outboundReference struct {
	Tag           string
	Protocol      string
	Status        string
	ProxyTag      string
	KernelSupport []string
}

// Resource mutations that change references (policies, outbounds, machines
// and nodes) use one transaction-scoped advisory lock.  Row locks alone do
// not prevent a policy/archive check from racing a concurrent insert/update
// in another table.  The lock is intentionally coarse: these admin writes
// are infrequent, and correctness is more important than parallel edits.
func lockResourceConsistencyTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('cpanel:resource-consistency', 0))`)
	return err
}

func normalizeMachineKernel(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "xray", nil
	}
	if value != "xray" && value != "singbox" {
		return "", ErrConflict
	}
	return value, nil
}

func normalizeMachineStatus(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	switch value {
	case "pending", "online", "offline", "disabled":
		return value, nil
	default:
		return "", ErrConflict
	}
}

func normalizeNodeStatus(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	switch value {
	case "draft", "disabled", "error":
		return value, nil
	case "published":
		// Published nodes require a revision/control event and must go through
		// PublishNode. Store callers must not bypass that lifecycle.
		return "", ErrConflict
	default:
		return "", ErrConflict
	}
}

func normalizeRoutePolicyStatus(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	switch value {
	case "published", "disabled":
		return value, nil
	default:
		return "", ErrConflict
	}
}

func normalizeKernelSupport(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, ErrOutboundKernelInvalid
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "xray" && value != "singbox" {
			return nil, ErrOutboundKernelInvalid
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func kernelSupported(support []string, kernel string) bool {
	for _, value := range support {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(kernel)) {
			return true
		}
	}
	return false
}

func normalizePolicySlot(slot string) string {
	switch strings.ToLower(strings.TrimSpace(slot)) {
	case "admin":
		return "admin"
	case "member":
		return "member"
	default:
		return "default"
	}
}

func (s *Store) validateNodeBindingsTx(ctx context.Context, tx pgx.Tx, machineID string, defaultPolicyID, adminPolicyID, memberPolicyID *string, kernelType string) error {
	kernelType = strings.ToLower(strings.TrimSpace(kernelType))
	if kernelType != "xray" && kernelType != "singbox" {
		return ErrConflict
	}
	var machineStatus string
	err := tx.QueryRow(ctx, `SELECT status FROM machines WHERE id=$1 FOR UPDATE`, machineID).Scan(&machineStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if machineStatus == "disabled" || machineStatus == "archived" {
		return ErrMachineUnavailable
	}
	bindings := []struct {
		slot     string
		policyID *string
	}{
		{slot: "default", policyID: defaultPolicyID},
		{slot: "admin", policyID: adminPolicyID},
		{slot: "member", policyID: memberPolicyID},
	}
	for _, binding := range bindings {
		slot, policyID := binding.slot, binding.policyID
		if policyID == nil || strings.TrimSpace(*policyID) == "" {
			continue
		}
		if err := s.validateRoutePolicyBindingTx(ctx, tx, strings.TrimSpace(*policyID), slot, []string{kernelType}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) validateRoutePolicyBindingTx(ctx context.Context, tx pgx.Tx, policyID, slot string, kernels []string) error {
	var status, scope, defaultTag string
	var rawRules []byte
	err := tx.QueryRow(ctx, `SELECT r.status,r.scope,COALESCE(r.default_outbound_tag,''),
		COALESCE((SELECT rr.rules FROM route_policy_revisions rr
			WHERE rr.route_policy_id=r.id ORDER BY rr.revision DESC LIMIT 1),'[]'::jsonb)
		FROM route_policies r WHERE r.id=$1 FOR SHARE`, policyID).Scan(&status, &scope, &defaultTag, &rawRules)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "published" {
		return ErrRoutePolicyUnavailable
	}
	if normalizeRoutePolicyScope(scope) != normalizePolicySlot(slot) {
		return ErrRoutePolicyScopeMismatch
	}
	var rules []domain.RoutePolicyRule
	if err := json.Unmarshal(rawRules, &rules); err != nil {
		return err
	}
	return validateRouteTargetsAgainstOutbounds(ctx, tx, rules, defaultTag, kernels)
}

func routePolicyTargets(rules []domain.RoutePolicyRule, defaultOutboundTag string) map[string]struct{} {
	targets := make(map[string]struct{})
	for _, rule := range rules {
		if rule.Disabled || strings.ToLower(strings.TrimSpace(rule.Action.Type)) != "route" {
			continue
		}
		if target := strings.ToLower(strings.TrimSpace(rule.Action.Target)); target != "" {
			targets[target] = struct{}{}
		}
	}
	if target := strings.ToLower(strings.TrimSpace(defaultOutboundTag)); target != "" {
		targets[target] = struct{}{}
	}
	return targets
}

func loadOutboundReferences(ctx context.Context, q resourceQuerier) (map[string]outboundReference, error) {
	rows, err := q.Query(ctx, `SELECT lower(tag),protocol,status,lower(NULLIF(proxy_tag,'')),kernel_support
		FROM outbounds WHERE status <> 'archived' FOR SHARE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make(map[string]outboundReference)
	for rows.Next() {
		var item outboundReference
		if err := rows.Scan(&item.Tag, &item.Protocol, &item.Status, &item.ProxyTag, &item.KernelSupport); err != nil {
			return nil, err
		}
		item.Tag = strings.ToLower(strings.TrimSpace(item.Tag))
		item.Protocol = strings.ToLower(strings.TrimSpace(item.Protocol))
		item.ProxyTag = strings.ToLower(strings.TrimSpace(item.ProxyTag))
		items[item.Tag] = item
	}
	return items, rows.Err()
}

func validateOutboundChain(tag string, outbounds map[string]outboundReference, kernels []string, visiting map[string]bool) error {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" || tag == "direct" || tag == "block" {
		return nil
	}
	if visiting[tag] {
		return ErrRouteOutboundUnavailable
	}
	item, ok := outbounds[tag]
	if !ok || item.Status != "active" || item.Protocol == "direct" || item.Protocol == "block" {
		return ErrRouteOutboundUnavailable
	}
	for _, kernel := range kernels {
		if !kernelSupported(item.KernelSupport, kernel) {
			return errors.Join(ErrRouteOutboundUnavailable, ErrOutboundKernelUnsupported)
		}
	}
	visiting[tag] = true
	err := validateOutboundChain(item.ProxyTag, outbounds, kernels, visiting)
	delete(visiting, tag)
	return err
}

func validateRouteTargetsAgainstOutbounds(ctx context.Context, q resourceQuerier, rules []domain.RoutePolicyRule, defaultOutboundTag string, kernels []string) error {
	targets := routePolicyTargets(rules, defaultOutboundTag)
	if len(targets) == 0 {
		return nil
	}
	outbounds, err := loadOutboundReferences(ctx, q)
	if err != nil {
		return err
	}
	for target := range targets {
		if err := validateOutboundChain(target, outbounds, kernels, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) routePolicyReferenceKernelsTx(ctx context.Context, tx pgx.Tx, policyID string) (map[string]struct{}, error) {
	rows, err := tx.Query(ctx, `SELECT n.kernel_type FROM nodes n
		WHERE n.status <> 'archived' AND $1 IN (n.route_policy_id,n.admin_route_policy_id,n.member_route_policy_id)
		FOR SHARE`, policyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	kernels := make(map[string]struct{})
	for rows.Next() {
		var kernel string
		if err := rows.Scan(&kernel); err != nil {
			return nil, err
		}
		kernels[strings.ToLower(strings.TrimSpace(kernel))] = struct{}{}
	}
	return kernels, rows.Err()
}

func kernelsSlice(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *Store) validateRoutePolicyMutationTx(ctx context.Context, tx pgx.Tx, policyID, resultingStatus, resultingScope, defaultOutboundTag string, rules []domain.RoutePolicyRule) error {
	kernels, err := s.routePolicyReferenceKernelsTx(ctx, tx, policyID)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT refs.slot FROM (
		SELECT n.id,'default' AS slot FROM nodes n WHERE n.status <> 'archived' AND n.route_policy_id=$1
		UNION ALL SELECT n.id,'admin' FROM nodes n WHERE n.status <> 'archived' AND n.admin_route_policy_id=$1
		UNION ALL SELECT n.id,'member' FROM nodes n WHERE n.status <> 'archived' AND n.member_route_policy_id=$1
	) refs JOIN nodes locked ON locked.id=refs.id FOR SHARE`, policyID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var slot string
		if err := rows.Scan(&slot); err != nil {
			return err
		}
		if normalizeRoutePolicyScope(resultingScope) != normalizePolicySlot(slot) {
			return ErrRoutePolicyScopeMismatch
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(kernels) > 0 && resultingStatus != "published" {
		return ErrRoutePolicyInUse
	}
	if resultingStatus != "published" {
		return nil
	}
	return validateRouteTargetsAgainstOutbounds(ctx, tx, rules, defaultOutboundTag, kernelsSlice(kernels))
}

func validateRouteTargetsAgainstOutboundsMap(rules []domain.RoutePolicyRule, defaultOutboundTag string, kernels []string, outbounds map[string]outboundReference) error {
	for target := range routePolicyTargets(rules, defaultOutboundTag) {
		if err := validateOutboundChain(target, outbounds, kernels, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) validateAllPublishedPoliciesForOutboundTx(ctx context.Context, tx pgx.Tx, replaced outboundReference, previousTag string) error {
	outbounds, err := loadOutboundReferences(ctx, tx)
	if err != nil {
		return err
	}
	if previousTag != "" && !strings.EqualFold(previousTag, replaced.Tag) {
		delete(outbounds, strings.ToLower(strings.TrimSpace(previousTag)))
	}
	outbounds[replaced.Tag] = replaced
	rows, err := tx.Query(ctx, `SELECT id,COALESCE(default_outbound_tag,''),
		COALESCE((SELECT rr.rules FROM route_policy_revisions rr
			WHERE rr.route_policy_id=r.id ORDER BY rr.revision DESC LIMIT 1),'[]'::jsonb)
		FROM route_policies r WHERE r.status='published' FOR SHARE`)
	if err != nil {
		return err
	}
	type policySnapshot struct {
		id, defaultTag string
		rules          []domain.RoutePolicyRule
	}
	policies := make([]policySnapshot, 0)
	for rows.Next() {
		var policyID, defaultTag string
		var rawRules []byte
		if err := rows.Scan(&policyID, &defaultTag, &rawRules); err != nil {
			return err
		}
		var rules []domain.RoutePolicyRule
		if err := json.Unmarshal(rawRules, &rules); err != nil {
			return err
		}
		policies = append(policies, policySnapshot{id: policyID, defaultTag: defaultTag, rules: rules})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, policy := range policies {
		kernels, err := s.routePolicyReferenceKernelsTx(ctx, tx, policy.id)
		if err != nil {
			return err
		}
		if err := validateRouteTargetsAgainstOutboundsMap(policy.rules, policy.defaultTag, kernelsSlice(kernels), outbounds); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) validateOutboundMutationTx(ctx context.Context, tx pgx.Tx, replaced outboundReference, previousTag string) error {
	if replaced.Tag == "" || replaced.Tag == "direct" || replaced.Tag == "block" {
		return ErrConflict
	}
	if replaced.Status != "active" && replaced.Status != "disabled" && replaced.Status != "archived" {
		return ErrConflict
	}
	outbounds, err := loadOutboundReferences(ctx, tx)
	if err != nil {
		return err
	}
	previousTag = strings.ToLower(strings.TrimSpace(previousTag))
	for tag, item := range outbounds {
		if tag != replaced.Tag && (item.ProxyTag == replaced.Tag || (previousTag != "" && item.ProxyTag == previousTag)) {
			return ErrOutboundInUse
		}
	}
	if previousTag != "" && previousTag != replaced.Tag {
		delete(outbounds, previousTag)
	}
	outbounds[replaced.Tag] = replaced
	if replaced.Status == "active" {
		if err := validateOutboundChain(replaced.Tag, outbounds, nil, map[string]bool{}); err != nil {
			return err
		}
	}
	return s.validateAllPublishedPoliciesForOutboundTx(ctx, tx, replaced, previousTag)
}

func (s *Store) outboundTagReferencedTx(ctx context.Context, tx pgx.Tx, tag string) (bool, error) {
	var referenced bool
	err := tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM outbounds o WHERE lower(o.proxy_tag)=lower($1))
		OR EXISTS(SELECT 1 FROM route_policies r WHERE r.status='published' AND lower(r.default_outbound_tag)=lower($1))
		OR EXISTS(SELECT 1 FROM route_policies r
			CROSS JOIN LATERAL (SELECT rr.rules FROM route_policy_revisions rr
				WHERE rr.route_policy_id=r.id ORDER BY rr.revision DESC LIMIT 1) latest
			WHERE r.status='published' AND EXISTS(SELECT 1 FROM jsonb_array_elements(latest.rules) rule
				WHERE COALESCE((rule->>'disabled')::boolean,false)=false
				  AND lower(COALESCE(rule->'action'->>'type',''))='route'
				  AND lower(COALESCE(rule->'action'->>'target',''))=lower($1))`, tag).Scan(&referenced)
	return referenced, err
}
