package management

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type listPageInfo struct {
	Total     int    `json:"total"`
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
}

func parseListSort(c *gin.Context, defaultBy, defaultOrder string, allowed map[string]struct{}) (string, string, error) {
	sortBy := strings.ToLower(strings.TrimSpace(c.Query("sort_by")))
	if sortBy == "" {
		sortBy = defaultBy
	}
	if _, ok := allowed[sortBy]; !ok {
		return "", "", fmt.Errorf("invalid sort_by")
	}

	sortOrder := strings.ToLower(strings.TrimSpace(c.Query("sort_order")))
	if sortOrder == "" {
		sortOrder = defaultOrder
	}
	switch sortOrder {
	case "asc", "desc":
	default:
		return "", "", fmt.Errorf("invalid sort_order")
	}

	return sortBy, sortOrder, nil
}

func parseListPageInfo[T any](c *gin.Context, items []T, sortBy, sortOrder string) ([]T, listPageInfo, error) {
	info := listPageInfo{
		Total:     len(items),
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	page, err := parsePositiveInt(strings.TrimSpace(c.Query("page")), 0)
	if err != nil {
		return nil, info, fmt.Errorf("invalid page")
	}
	pageSize, err := parsePositiveInt(strings.TrimSpace(c.Query("page_size")), 0)
	if err != nil {
		return nil, info, fmt.Errorf("invalid page_size")
	}

	if pageSize == 0 {
		return items, info, nil
	}
	if page == 0 {
		page = 1
	}

	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	info.Page = page
	info.PageSize = pageSize
	return items[start:end], info, nil
}

func parsePositiveInt(raw string, zeroValue int) (int, error) {
	if raw == "" {
		return zeroValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid integer")
	}
	return value, nil
}

func compareStringsFold(left, right string) int {
	l := strings.ToLower(strings.TrimSpace(left))
	r := strings.ToLower(strings.TrimSpace(right))
	switch {
	case l < r:
		return -1
	case l > r:
		return 1
	default:
		return 0
	}
}

func compareBools(left, right bool) int {
	switch {
	case left == right:
		return 0
	case !left && right:
		return -1
	default:
		return 1
	}
}

func compareInt64s(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInts(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareFloat64s(left, right float64) int {
	switch {
	case math.Abs(left-right) <= 1e-9:
		return 0
	case left < right:
		return -1
	default:
		return 1
	}
}

func compareTimes(left, right time.Time) int {
	left = left.UTC()
	right = right.UTC()
	switch {
	case left.Before(right):
		return -1
	case left.After(right):
		return 1
	default:
		return 0
	}
}

func listValueAsString(entry gin.H, keys ...string) string {
	for _, key := range keys {
		raw, ok := entry[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(value.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func listValueAsInt(entry gin.H, keys ...string) int {
	for _, key := range keys {
		raw, ok := entry[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case int:
			return value
		case int64:
			return int(value)
		case float64:
			return int(value)
		case json.Number:
			if parsed, err := strconv.Atoi(string(value)); err == nil {
				return parsed
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func listValueAsInt64(entry gin.H, keys ...string) int64 {
	for _, key := range keys {
		raw, ok := entry[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case int:
			return int64(value)
		case int64:
			return value
		case float64:
			return int64(value)
		case json.Number:
			if parsed, err := strconv.ParseInt(string(value), 10, 64); err == nil {
				return parsed
			}
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func listValueAsTime(entry gin.H, keys ...string) time.Time {
	for _, key := range keys {
		raw, ok := entry[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case time.Time:
			return value
		case *time.Time:
			if value != nil {
				return *value
			}
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
				return parsed
			}
			if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}
