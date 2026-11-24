package store

import (
	"testing"
	"time"
)

func TestOverviewWeekUsesSiteDayAndCombinedTraffic(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	seedEntitlements(t, pool, "never", 0, 0)
	s := New(pool)
	user := quotaTestUser(t, s, "overview", nil)
	if _, err := pool.Exec(ctx, `INSERT INTO settings(section,key,value) VALUES('site','timezone','"Pacific/Kiritimati"')
		ON CONFLICT(section,key) DO UPDATE SET value=EXCLUDED.value`); err != nil {
		t.Fatal(err)
	}
	day := time.Now().In(loadTrafficLocation("Pacific/Kiritimati")).Format(time.DateOnly)
	if _, err := pool.Exec(ctx, `INSERT INTO traffic_daily(day,user_id,node_id,upload_bytes,download_bytes)
		VALUES($1::date,$2,'nod_quota',21,30),($1::date-6,$2,'nod_quota',0,0),
		($1::date-7,$2,'nod_quota',1000,1000),($1::date+1,$2,'nod_quota',2000,2000)`, day, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO imported_user_traffic_daily(day,user_id,upload_bytes,download_bytes) VALUES($1::date,$2,10,20)`, day, user.ID); err != nil {
		t.Fatal(err)
	}
	overview, err := s.Overview(ctx, "yesterday")
	if err != nil {
		t.Fatal(err)
	}
	if overview.TrafficDay != day || overview.TrafficTodayUpload != 31 || overview.TrafficTodayDownload != 50 || overview.TrafficToday != 81 {
		t.Fatalf("unexpected today totals %+v", overview)
	}
	if len(overview.TrafficWeek) != 2 || overview.TrafficWeek[0].Day.Format(time.DateOnly) != day || overview.TrafficWeek[1].UploadBytes != 0 {
		t.Fatalf("unexpected daily samples %+v", overview.TrafficWeek)
	}
}
