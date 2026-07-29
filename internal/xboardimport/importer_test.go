package xboardimport

import (
	"reflect"
	"testing"
	"time"
)

func TestIntListAcceptsStringAndNumberIDs(t *testing.T) {
	want := []int64{3, 2, 1, 4}
	if got := intList(`["3",2,"1",4]`); !reflect.DeepEqual(got, want) {
		t.Fatalf("intList() = %v, want %v", got, want)
	}
}

func TestRootNodeIDFollowsParentChain(t *testing.T) {
	nodes := map[int64]sourceNode{
		55: {ID: 55},
		56: {ID: 56, ParentID: 55},
		57: {ID: 57, ParentID: 56},
	}
	if got := rootNodeID(57, nodes); got != 55 {
		t.Fatalf("rootNodeID() = %d, want 55", got)
	}
}

func TestNullableSourceTime(t *testing.T) {
	if got := nullableSourceTime(int64(0)); got != nil {
		t.Fatalf("nullableSourceTime(0) = %v, want nil", got)
	}
	want := time.Date(2026, 7, 29, 12, 30, 0, 0, time.UTC)
	got := nullableSourceTime("2026-07-29 12:30:00")
	if got == nil || !got.Equal(want) {
		t.Fatalf("nullableSourceTime() = %v, want %v", got, want)
	}
}
