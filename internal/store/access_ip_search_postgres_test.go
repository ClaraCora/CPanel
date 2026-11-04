package store

import (
	"fmt"
	"testing"

	"cpanel/internal/domain"
)

func TestAccessIPSearchPaginatesOnlyAccountsWithRecordsPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	s := New(pool)
	for index := 0; index < 55; index++ {
		if _, err := s.CreateUser(ctx, domain.UserCreate{Role: "user", Name: fmt.Sprintf("A%03d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < 3; index++ {
		item, err := s.CreateUser(ctx, domain.UserCreate{Role: "friend", Name: fmt.Sprintf("Z%03d", index)})
		if err != nil {
			t.Fatal(err)
		}
		city, nodeName, isp := "北京", "日本入口", "运营商乙"
		if index == 0 {
			city, nodeName, isp = "深圳", "香港入口", "运营商甲"
		}
		if _, err := pool.Exec(ctx, `INSERT INTO user_access_ips(user_id,ip_address,first_seen_at,last_seen_at,last_node_name,
			location_scope,location_country,location_province,location_city,location_isp,location_updated_at)
			VALUES($1,$2,now(),now(),$3,'public','中国','广东',$4,$5,now())`, item.ID, fmt.Sprintf("192.0.2.%d", index+1), nodeName, city, isp); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		page                 int
		query, role          string
		wantItems, wantTotal int
	}{
		{1, "", "", 2, 3}, {2, "", "", 1, 3}, {3, "", "", 0, 3}, {1, "深圳", "", 1, 1}, {1, "香港入口", "", 1, 1},
		{1, "运营商甲", "friend", 1, 1}, {1, "192.0.2.1", "", 1, 1}, {1, "深圳", "user", 0, 0}, {1, "不存在", "", 0, 0},
	} {
		items, total, err := s.ListUserAccessIPsPage(ctx, test.page, 2, test.query, "name", "asc", test.role)
		if err != nil || total != test.wantTotal || len(items) != test.wantItems {
			t.Fatalf("page=%d q=%q role=%q items=%d total=%d err=%v", test.page, test.query, test.role, len(items), total, err)
		}
		for _, item := range items {
			if len(item.Addresses) == 0 {
				t.Fatal("returned account without access IP", item.UserID)
			}
		}
	}
}
