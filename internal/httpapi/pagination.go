package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

// listPage contains the common pagination envelope used by growing admin lists.
// Legacy callers that do not provide pagination parameters continue to receive
// the original array response for compatibility with older bundled clients.
type listPage[T any] struct {
	Items    []T  `json:"items"`
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	Total    int  `json:"total"`
	HasMore  bool `json:"has_more"`
}

type listQuery struct {
	Page      int
	PageSize  int
	Query     string
	Sort      string
	Order     string
	Paginated bool
}

func parseListQuery(r *http.Request, defaultSort string, allowedSort map[string]bool) listQuery {
	q := r.URL.Query()
	result := listQuery{Page: 1, PageSize: 50, Sort: defaultSort, Order: "desc"}
	result.Paginated = q.Has("page") || q.Has("page_size") || q.Has("q") || q.Has("sort") || q.Has("order")
	if value, err := strconv.Atoi(q.Get("page")); err == nil && value > 0 {
		result.Page = value
	}
	if value, err := strconv.Atoi(q.Get("page_size")); err == nil && value > 0 {
		result.PageSize = value
	}
	if result.PageSize > 100 {
		result.PageSize = 100
	}
	result.Query = strings.TrimSpace(q.Get("q"))
	if len([]rune(result.Query)) > 100 {
		result.Query = string([]rune(result.Query)[:100])
	}
	if allowedSort[q.Get("sort")] {
		result.Sort = q.Get("sort")
	}
	if strings.EqualFold(q.Get("order"), "asc") {
		result.Order = "asc"
	}
	return result
}
