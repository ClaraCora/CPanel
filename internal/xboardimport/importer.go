package xboardimport

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"
)

type Options struct {
	SourcePath string
	FriendIDs  map[int64]bool
	SkipNodeID map[int64]bool
}

type Result struct {
	Machines     int `json:"machines"`
	Nodes        int `json:"nodes"`
	Endpoints    int `json:"endpoints"`
	AccessGroups int `json:"access_groups"`
	Plans        int `json:"plans"`
	Users        int `json:"users"`
	Routes       int `json:"routes"`
	NodeStats    int `json:"node_stats"`
	UserStats    int `json:"user_stats"`
}

type Importer struct{ target *pgxpool.Pool }

func New(target *pgxpool.Pool) *Importer { return &Importer{target: target} }

func (i *Importer) Run(ctx context.Context, options Options) (Result, error) {
	if strings.TrimSpace(options.SourcePath) == "" {
		return Result{}, fmt.Errorf("source SQLite path is required")
	}
	absolutePath, err := filepath.Abs(options.SourcePath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve Xboard database path: %w", err)
	}
	sourceURL := url.URL{Scheme: "file", Path: filepath.ToSlash(absolutePath), RawQuery: "mode=ro"}
	source, err := sql.Open("sqlite", sourceURL.String())
	if err != nil {
		return Result{}, fmt.Errorf("open Xboard database: %w", err)
	}
	defer source.Close()
	if err := source.PingContext(ctx); err != nil {
		return Result{}, fmt.Errorf("read Xboard database: %w", err)
	}

	tx, err := i.target.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	result := Result{}
	machineHosts, err := rootMachineHosts(ctx, source, options.SkipNodeID)
	if err != nil {
		return result, err
	}
	if result.Machines, err = importMachines(ctx, source, tx, machineHosts); err != nil {
		return result, err
	}
	routeCount, validRoutes, err := importRoutes(ctx, source, tx)
	if err != nil {
		return result, err
	}
	result.Routes = routeCount
	nodeParents, importedRoots, err := importNodes(ctx, source, tx, options.SkipNodeID, validRoutes, &result)
	if err != nil {
		return result, err
	}
	groupCount, validGroups, err := importGroups(ctx, source, tx, nodeParents, importedRoots)
	if err != nil {
		return result, err
	}
	result.AccessGroups = groupCount
	planCount, validPlans, err := importPlans(ctx, source, tx, validGroups)
	if err != nil {
		return result, err
	}
	result.Plans = planCount
	if result.Users, err = importUsers(ctx, source, tx, options.FriendIDs, validGroups, validPlans); err != nil {
		return result, err
	}
	if result.NodeStats, err = importNodeStats(ctx, source, tx, nodeParents, importedRoots); err != nil {
		return result, err
	}
	if result.UserStats, err = importUserStats(ctx, source, tx); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

// BackfillSubscriptionTokens restores only the recoverable Xboard subscription tokens.
// It intentionally leaves every other imported resource and any already-saved token unchanged.
func (i *Importer) BackfillSubscriptionTokens(ctx context.Context, sourcePath string) (int, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return 0, fmt.Errorf("source SQLite path is required")
	}
	absolutePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("resolve Xboard database path: %w", err)
	}
	sourceURL := url.URL{Scheme: "file", Path: filepath.ToSlash(absolutePath), RawQuery: "mode=ro"}
	source, err := sql.Open("sqlite", sourceURL.String())
	if err != nil {
		return 0, fmt.Errorf("open Xboard database: %w", err)
	}
	defer source.Close()
	if err := source.PingContext(ctx); err != nil {
		return 0, fmt.Errorf("read Xboard database: %w", err)
	}

	rows, err := source.QueryContext(ctx, `SELECT id,token FROM v2_user WHERE is_admin=0 ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	tx, err := i.target.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	count := 0
	for rows.Next() {
		var id int64
		var token string
		if err := rows.Scan(&id, &token); err != nil {
			return count, err
		}
		if strings.TrimSpace(token) == "" {
			continue
		}
		digest := sha256.Sum256([]byte(token))
		prefix := token
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		command, err := tx.Exec(ctx, `UPDATE users SET subscription_token_plain=$2,
			subscription_token_hash=$3,subscription_token_prefix=$4,updated_at=now()
			WHERE id=$1 AND status <> 'archived' AND COALESCE(subscription_token_plain,'')=''`,
			userID(id), token, digest[:], prefix)
		if err != nil {
			return count, err
		}
		count += int(command.RowsAffected())
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	if err := tx.Commit(ctx); err != nil {
		return count, err
	}
	return count, nil
}

type sourceNode struct {
	ID               int64
	ParentID         int64
	MachineID        int64
	Type             string
	GroupIDs         string
	RouteIDs         string
	Name             string
	Host             string
	Port             string
	ServerPort       int64
	ProtocolSettings string
	Show             int64
	Enabled          int64
	CreatedAt        any
	UpdatedAt        any
}

func rootMachineHosts(ctx context.Context, source *sql.DB, skipped map[int64]bool) (map[int64]string, error) {
	rows, err := source.QueryContext(ctx, `SELECT machine_id,host,id FROM v2_server WHERE parent_id=0 AND machine_id IS NOT NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int64]string{}
	for rows.Next() {
		var machineID, nodeID int64
		var host string
		if err := rows.Scan(&machineID, &host, &nodeID); err != nil {
			return nil, err
		}
		if !skipped[nodeID] && result[machineID] == "" {
			result[machineID] = host
		}
	}
	return result, rows.Err()
}

func importMachines(ctx context.Context, source *sql.DB, tx pgx.Tx, hosts map[int64]string) (int, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,name,COALESCE(notes,''),created_at,updated_at FROM v2_server_machine ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		var name, notes string
		var created, updated any
		if err := rows.Scan(&id, &name, &notes, &created, &updated); err != nil {
			return count, err
		}
		host, wanted := hosts[id]
		if !wanted {
			continue
		}
		_, err = tx.Exec(ctx, `INSERT INTO machines(id,name,host,notes,status,kernel_type,created_at,updated_at)
			VALUES($1,$2,$3,$4,'pending','singbox',$5,$6) ON CONFLICT(id) DO UPDATE SET
			name=EXCLUDED.name,host=EXCLUDED.host,notes=EXCLUDED.notes,status='pending',updated_at=EXCLUDED.updated_at`,
			machineID(id), name, host, notes, sourceTimeOrNow(created), sourceTimeOrNow(updated))
		if err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func importRoutes(ctx context.Context, source *sql.DB, tx pgx.Tx) (int, map[int64]bool, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,remarks,COALESCE(match,'[]'),action,COALESCE(action_value,''),created_at,updated_at FROM v2_server_route ORDER BY id`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	count := 0
	valid := map[int64]bool{}
	for rows.Next() {
		var id int64
		var name, match, action, actionValue string
		var created, updated any
		if err := rows.Scan(&id, &name, &match, &action, &actionValue, &created, &updated); err != nil {
			return count, nil, err
		}
		matchValue := any(match)
		if json.Valid([]byte(match)) {
			matchValue = json.RawMessage(match)
		}
		notes, err := json.Marshal(map[string]any{"xboard_id": id, "match": matchValue, "action": action, "action_value": actionValue})
		if err != nil {
			return count, nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO route_policies(id,name,status,notes,created_at,updated_at) VALUES($1,$2,'draft',$3,$4,$5)
			ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,status='draft',notes=EXCLUDED.notes,updated_at=EXCLUDED.updated_at`,
			routeID(id), name, string(notes), sourceTimeOrNow(created), sourceTimeOrNow(updated))
		if err != nil {
			return count, nil, err
		}
		valid[id] = true
		count++
	}
	return count, valid, rows.Err()
}

func importNodes(ctx context.Context, source *sql.DB, tx pgx.Tx, skipped map[int64]bool, validRoutes map[int64]bool, result *Result) (map[int64]int64, map[int64]bool, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,parent_id,COALESCE(machine_id,0),type,COALESCE(group_ids,'[]'),COALESCE(route_ids,'[]'),name,host,COALESCE(port,''),server_port,COALESCE(protocol_settings,'{}'),show,enabled,created_at,updated_at FROM v2_server ORDER BY parent_id,id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	all := make([]sourceNode, 0)
	byID := map[int64]sourceNode{}
	for rows.Next() {
		var item sourceNode
		if err := rows.Scan(&item.ID, &item.ParentID, &item.MachineID, &item.Type, &item.GroupIDs, &item.RouteIDs, &item.Name, &item.Host, &item.Port, &item.ServerPort, &item.ProtocolSettings, &item.Show, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, nil, err
		}
		all = append(all, item)
		byID[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	parents := make(map[int64]int64, len(all))
	for _, item := range all {
		parents[item.ID] = rootNodeID(item.ID, byID)
	}
	importedRoots := map[int64]bool{}
	for _, item := range all {
		if item.ParentID != 0 || item.MachineID == 0 || skipped[item.ID] {
			continue
		}
		var route *string
		for _, id := range intList(item.RouteIDs) {
			if validRoutes[id] {
				value := routeID(id)
				route = &value
				break
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO nodes(id,machine_id,route_policy_id,name,protocol,listen_ip,server_port,kernel_type,config,status,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,'0.0.0.0',$6,'singbox',$7,'draft',$8,$9) ON CONFLICT(id) DO UPDATE SET
			machine_id=EXCLUDED.machine_id,route_policy_id=EXCLUDED.route_policy_id,name=EXCLUDED.name,protocol=EXCLUDED.protocol,
			server_port=EXCLUDED.server_port,config=EXCLUDED.config,status='draft',updated_at=EXCLUDED.updated_at`,
			nodeID(item.ID), machineID(item.MachineID), route, item.Name, strings.ToLower(item.Type), item.ServerPort, validJSON(item.ProtocolSettings), sourceTimeOrNow(item.CreatedAt), sourceTimeOrNow(item.UpdatedAt))
		if err != nil {
			return nil, nil, err
		}
		importedRoots[item.ID] = true
		result.Nodes++
	}

	for rootID := range importedRoots {
		if _, err := tx.Exec(ctx, `DELETE FROM node_endpoints WHERE node_id=$1 AND id LIKE 'ep_xb_%'`, nodeID(rootID)); err != nil {
			return nil, nil, err
		}
	}
	for _, item := range all {
		rootID := parents[item.ID]
		if rootID == 0 || skipped[rootID] || !importedRoots[rootID] {
			continue
		}
		port, _ := strconv.Atoi(strings.TrimSpace(item.Port))
		if port == 0 {
			port = int(item.ServerPort)
		}
		if port < 1 || port > 65535 || strings.TrimSpace(item.Host) == "" {
			return nil, nil, fmt.Errorf("Xboard node %d has invalid endpoint %q:%d", item.ID, item.Host, port)
		}
		status := "active"
		if item.Show == 0 || item.Enabled == 0 {
			status = "disabled"
		}
		_, err := tx.Exec(ctx, `INSERT INTO node_endpoints(id,node_id,name,host,port,status,sort_order,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, endpointID(item.ID), nodeID(rootID), item.Name, item.Host, port, status, item.ID, sourceTimeOrNow(item.CreatedAt), sourceTimeOrNow(item.UpdatedAt))
		if err != nil {
			return nil, nil, err
		}
		result.Endpoints++
	}
	return parents, importedRoots, nil
}

func importGroups(ctx context.Context, source *sql.DB, tx pgx.Tx, parents map[int64]int64, importedRoots map[int64]bool) (int, map[int64]bool, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,name,created_at,updated_at FROM v2_server_group ORDER BY id`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	count := 0
	valid := map[int64]bool{}
	for rows.Next() {
		var id int64
		var name string
		var created, updated any
		if err := rows.Scan(&id, &name, &created, &updated); err != nil {
			return count, nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO access_groups(id,name,status,notes,created_at,updated_at) VALUES($1,$2,'active',$3,$4,$5)
			ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,notes=EXCLUDED.notes,updated_at=EXCLUDED.updated_at`, groupID(id), name, fmt.Sprintf("从 Xboard 权限组 %d 导入", id), sourceTimeOrNow(created), sourceTimeOrNow(updated))
		if err != nil {
			return count, nil, err
		}
		valid[id] = true
		count++
	}
	if err := rows.Err(); err != nil {
		return count, nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM access_group_nodes WHERE access_group_id LIKE 'grp_xb_%' AND node_id LIKE 'nod_xb_%'`); err != nil {
		return count, nil, err
	}
	serverRows, err := source.QueryContext(ctx, `SELECT id,group_ids FROM v2_server WHERE parent_id=0 AND machine_id IS NOT NULL`)
	if err != nil {
		return count, nil, err
	}
	defer serverRows.Close()
	for serverRows.Next() {
		var id int64
		var groups string
		if err := serverRows.Scan(&id, &groups); err != nil {
			return count, nil, err
		}
		rootID := parents[id]
		if rootID == 0 || !importedRoots[rootID] {
			continue
		}
		for _, group := range intList(groups) {
			if !valid[group] {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO access_group_nodes(access_group_id,node_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, groupID(group), nodeID(rootID)); err != nil {
				return count, nil, err
			}
		}
	}
	return count, valid, serverRows.Err()
}

func importPlans(ctx context.Context, source *sql.DB, tx pgx.Tx, validGroups map[int64]bool) (int, map[int64]bool, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,group_id,transfer_enable,name,COALESCE(speed_limit,0),COALESCE(device_limit,0),created_at,updated_at FROM v2_plan ORDER BY id`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	count := 0
	valid := map[int64]bool{}
	for rows.Next() {
		var id, group, trafficGB, speed, devices int64
		var name string
		var created, updated any
		if err := rows.Scan(&id, &group, &trafficGB, &name, &speed, &devices, &created, &updated); err != nil {
			return count, nil, err
		}
		if !validGroups[group] {
			return count, nil, fmt.Errorf("Xboard plan %d references missing group %d", id, group)
		}
		_, err = tx.Exec(ctx, `INSERT INTO plans(id,access_group_id,name,status,traffic_limit_bytes,speed_limit_mbps,device_limit,default_valid_days,notes,created_at,updated_at)
			VALUES($1,$2,$3,'active',$4,$5,$6,0,$7,$8,$9) ON CONFLICT(id) DO UPDATE SET access_group_id=EXCLUDED.access_group_id,name=EXCLUDED.name,
			traffic_limit_bytes=EXCLUDED.traffic_limit_bytes,speed_limit_mbps=EXCLUDED.speed_limit_mbps,device_limit=EXCLUDED.device_limit,notes=EXCLUDED.notes,updated_at=EXCLUDED.updated_at`,
			planID(id), groupID(group), name, trafficGB*1024*1024*1024, speed, devices, fmt.Sprintf("从 Xboard 套餐 %d 导入", id), sourceTimeOrNow(created), sourceTimeOrNow(updated))
		if err != nil {
			return count, nil, err
		}
		valid[id] = true
		count++
	}
	return count, valid, rows.Err()
}

func importUsers(ctx context.Context, source *sql.DB, tx pgx.Tx, friends, validGroups, validPlans map[int64]bool) (int, error) {
	rows, err := source.QueryContext(ctx, `SELECT id,email,token,uuid,group_id,plan_id,COALESCE(speed_limit,0),COALESCE(device_limit,0),u,d,transfer_enable,banned,expired_at,COALESCE(remarks,''),created_at,updated_at,next_reset_at FROM v2_user WHERE is_admin=0 ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, group, plan, speed, devices, upload, download, limit, banned int64
		var email, token, uuid, notes string
		var expires, created, updated, reset any
		if err := rows.Scan(&id, &email, &token, &uuid, &group, &plan, &speed, &devices, &upload, &download, &limit, &banned, &expires, &notes, &created, &updated, &reset); err != nil {
			return count, err
		}
		role := "user"
		if friends[id] {
			role = "friend"
		}
		status := "active"
		if banned != 0 {
			status = "paused"
		}
		var importedPlan *string
		if validPlans[plan] {
			value := planID(plan)
			importedPlan = &value
		}
		var importedGroup *string
		if validGroups[group] {
			value := groupID(group)
			importedGroup = &value
		}
		digest := sha256.Sum256([]byte(token))
		prefix := token
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		_, err = tx.Exec(ctx, `INSERT INTO users(id,role,plan_id,access_group_override_id,name,email,uuid,subscription_token_hash,subscription_token_prefix,subscription_token_plain,status,
			traffic_limit_override_bytes,speed_limit_override_mbps,device_limit_override,traffic_used_bytes,traffic_reset_at,expires_at,notes,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12::bigint,0),NULLIF($13::integer,0),NULLIF($14::integer,0),$15,$16,$17,$18,$19,$20)
			ON CONFLICT(id) DO UPDATE SET role=EXCLUDED.role,plan_id=EXCLUDED.plan_id,access_group_override_id=EXCLUDED.access_group_override_id,
			name=EXCLUDED.name,email=EXCLUDED.email,uuid=EXCLUDED.uuid,subscription_token_hash=EXCLUDED.subscription_token_hash,
			subscription_token_prefix=EXCLUDED.subscription_token_prefix,subscription_token_plain=EXCLUDED.subscription_token_plain,
			status=EXCLUDED.status,traffic_limit_override_bytes=EXCLUDED.traffic_limit_override_bytes,
			speed_limit_override_mbps=EXCLUDED.speed_limit_override_mbps,device_limit_override=EXCLUDED.device_limit_override,
			traffic_used_bytes=EXCLUDED.traffic_used_bytes,traffic_reset_at=EXCLUDED.traffic_reset_at,expires_at=EXCLUDED.expires_at,notes=EXCLUDED.notes,updated_at=EXCLUDED.updated_at`,
			userID(id), role, importedPlan, importedGroup, email, email, uuid, digest[:], prefix, token, status, limit, speed, devices, upload+download,
			nullableSourceTime(reset), nullableSourceTime(expires), notes, sourceTimeOrNow(created), sourceTimeOrNow(updated))
		if err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func importNodeStats(ctx context.Context, source *sql.DB, tx pgx.Tx, parents map[int64]int64, importedRoots map[int64]bool) (int, error) {
	rows, err := source.QueryContext(ctx, `SELECT server_id,u,d,record_at FROM v2_stat_server WHERE record_type='d' ORDER BY record_at,server_id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type dailyTraffic struct {
		rootID   int64
		day      time.Time
		upload   int64
		download int64
	}
	totals := map[string]dailyTraffic{}
	for rows.Next() {
		var id, upload, download int64
		var at any
		if err := rows.Scan(&id, &upload, &download, &at); err != nil {
			return 0, err
		}
		root := parents[id]
		if root == 0 || !importedRoots[root] {
			continue
		}
		day := nullableSourceTime(at)
		if day == nil {
			continue
		}
		key := fmt.Sprintf("%d/%s", root, day.Format("2006-01-02"))
		total := totals[key]
		total.rootID = root
		total.day = *day
		total.upload += upload
		total.download += download
		totals[key] = total
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, total := range totals {
		if _, err := tx.Exec(ctx, `INSERT INTO imported_node_traffic_daily(day,node_id,upload_bytes,download_bytes) VALUES($1,$2,$3,$4)
			ON CONFLICT(day,node_id) DO UPDATE SET upload_bytes=EXCLUDED.upload_bytes,download_bytes=EXCLUDED.download_bytes`, total.day, nodeID(total.rootID), total.upload, total.download); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func importUserStats(ctx context.Context, source *sql.DB, tx pgx.Tx) (int, error) {
	rows, err := source.QueryContext(ctx, `SELECT user_id,SUM(u),SUM(d),record_at FROM v2_stat_user WHERE record_type='d' AND user_id<>1 GROUP BY record_at,user_id ORDER BY record_at,user_id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, upload, download int64
		var at any
		if err := rows.Scan(&id, &upload, &download, &at); err != nil {
			return count, err
		}
		day := nullableSourceTime(at)
		if day == nil {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO imported_user_traffic_daily(day,user_id,upload_bytes,download_bytes) VALUES($1,$2,$3,$4)
			ON CONFLICT(day,user_id) DO UPDATE SET upload_bytes=EXCLUDED.upload_bytes,download_bytes=EXCLUDED.download_bytes`, *day, userID(id), upload, download); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func rootNodeID(id int64, nodes map[int64]sourceNode) int64 {
	seen := map[int64]bool{}
	for id != 0 && !seen[id] {
		seen[id] = true
		node, ok := nodes[id]
		if !ok {
			return 0
		}
		if node.ParentID == 0 {
			return id
		}
		id = node.ParentID
	}
	return 0
}

func intList(value string) []int64 {
	var raw []any
	if json.Unmarshal([]byte(value), &raw) != nil {
		return nil
	}
	result := make([]int64, 0, len(raw))
	for _, value := range raw {
		var number int64
		var err error
		switch typed := value.(type) {
		case float64:
			number = int64(typed)
			if typed != float64(number) {
				continue
			}
		case string:
			number, err = strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		default:
			continue
		}
		if err == nil {
			result = append(result, number)
		}
	}
	return result
}

func validJSON(value string) json.RawMessage {
	if json.Valid([]byte(value)) {
		return json.RawMessage(value)
	}
	return json.RawMessage(`{}`)
}

func sourceTimeOrNow(value any) time.Time {
	if parsed := nullableSourceTime(value); parsed != nil {
		return *parsed
	}
	return time.Now().UTC()
}

func nullableSourceTime(value any) *time.Time {
	var parsed time.Time
	switch typed := value.(type) {
	case nil:
		return nil
	case time.Time:
		parsed = typed
	case int64:
		if typed <= 0 {
			return nil
		}
		parsed = time.Unix(typed, 0)
	case float64:
		if typed <= 0 {
			return nil
		}
		parsed = time.Unix(int64(typed), 0)
	case []byte:
		return nullableSourceTime(string(typed))
	case string:
		text := strings.TrimSpace(typed)
		if text == "" || text == "0" {
			return nil
		}
		if seconds, err := strconv.ParseInt(text, 10, 64); err == nil {
			if seconds <= 0 {
				return nil
			}
			parsed = time.Unix(seconds, 0)
			break
		}
		for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
			candidate, err := time.ParseInLocation(layout, text, time.UTC)
			if err == nil {
				parsed = candidate
				break
			}
		}
		if parsed.IsZero() {
			return nil
		}
	default:
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func machineID(id int64) string  { return fmt.Sprintf("mch_xb_%d", id) }
func nodeID(id int64) string     { return fmt.Sprintf("nod_xb_%d", id) }
func endpointID(id int64) string { return fmt.Sprintf("ep_xb_%d", id) }
func groupID(id int64) string    { return fmt.Sprintf("grp_xb_%d", id) }
func planID(id int64) string     { return fmt.Sprintf("pln_xb_%d", id) }
func userID(id int64) string     { return fmt.Sprintf("usr_xb_%d", id) }
func routeID(id int64) string    { return fmt.Sprintf("rte_xb_%d", id) }
