package store

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAuditChineseActionSearchPaginationPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	if _, err := pool.Exec(ctx, `INSERT INTO admins(id,email,name,password_hash) VALUES('adm_audit_search','audit@example.test','审计管理员','test-hash')`); err != nil {
		t.Fatal(err)
	}
	actions := []string{
		"node.create", "node.update", "node.publish", "node.restore", "node.delete",
		"outbound.create", "outbound.update", "outbound.delete",
		"plan.update", "access_group.delete", "route_policy.update", "admin.login", "vendor.rotate",
	}
	for index, action := range actions {
		resource := strings.SplitN(action, ".", 2)[0]
		resourceID, requestID := "resource_"+action, "request_"+action
		if action == "vendor.rotate" {
			resourceID, requestID = "unique-resource-id", "request-correlation"
		}
		if _, err := pool.Exec(ctx, `INSERT INTO audit_events(id,admin_id,action,resource_type,resource_id,request_id,created_at)
			VALUES($1,'adm_audit_search',$2,$3,$4,$5,$6)`, "audit_"+action, action, resource, resourceID, requestID,
			time.Date(2026, 11, 4, 0, index, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	s := New(pool)
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"节点", actions[:5]},
		{" 节点 ", actions[:5]},
		{"恢复", []string{"node.restore"}},
		{"出站", actions[5:8]},
		{"编辑", []string{"node.update", "outbound.update", "plan.update", "route_policy.update"}},
		{"权限组", []string{"access_group.delete"}},
		{"node.", actions[:5]},
		{"OUTBOUND", actions[5:8]},
		{"unique-resource-id", []string{"vendor.rotate"}},
		{"request-correlation", []string{"vendor.rotate"}},
		{"vendor.rotate", []string{"vendor.rotate"}},
		{"审计管理员", actions},
		{"不存在的操作", []string{}},
	} {
		t.Run(test.query, func(t *testing.T) {
			seen := make([]string, 0)
			const pageSize = 2
			for page := 1; page <= (len(test.want)+pageSize-1)/pageSize+1; page++ {
				items, total, err := s.ListAuditEventsPage(ctx, page, pageSize, test.query, "created_at", "asc")
				if err != nil {
					t.Fatal(err)
				}
				if total != len(test.want) {
					t.Fatalf("page %d total = %d, want %d", page, total, len(test.want))
				}
				if len(items) > pageSize {
					t.Fatalf("page %d returned %d records", page, len(items))
				}
				for _, item := range items {
					seen = append(seen, item.Action)
				}
			}
			if !reflect.DeepEqual(seen, test.want) {
				t.Fatalf("paged actions = %v, want %v", seen, test.want)
			}
		})
	}
}
