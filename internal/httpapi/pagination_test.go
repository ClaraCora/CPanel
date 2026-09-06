package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListQueryBoundsAndWhitelist(t *testing.T) {
	longQuery := strings.Repeat("x", 160)
	request := httptest.NewRequest("GET", "/?page=0&page_size=1000&q="+longQuery+"&sort=invalid&order=asc", nil)
	result := parseListQuery(request, "created_at", map[string]bool{"created_at": true})
	if result.Page != 1 || result.PageSize != 100 || result.Sort != "created_at" || result.Order != "asc" {
		t.Fatalf("unexpected pagination query: %#v", result)
	}
	if len([]rune(result.Query)) != 100 {
		t.Fatalf("query length = %d, want 100", len([]rune(result.Query)))
	}
}
