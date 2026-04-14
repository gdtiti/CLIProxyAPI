package management

import (
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type codexRefreshHistoryFilters struct {
	Query     string
	AuthID    string
	AuthIndex string
	Email     string
	Result    string
	Trigger   string
}

// ListCodexAuthRefreshHistory returns persisted automatic refresh history for Codex auths.
func (h *Handler) ListCodexAuthRefreshHistory(c *gin.Context) {
	items := make([]gin.H, 0)
	if h != nil && h.authManager != nil {
		items = buildCodexRefreshHistoryEntries(h.authManager.List())
	}
	items = filterCodexRefreshHistoryEntries(items, buildCodexRefreshHistoryFilters(c))
	sortCodexRefreshHistoryEntries(items, c.Query("sort_by"), c.Query("sort_order"))
	items, pagination := paginateAuthFileEntries(items, parseAuthFilesListOffset(c.Query("offset")), parseCodexRefreshHistoryLimit(c.Query("limit")))
	c.JSON(200, gin.H{"items": items, "pagination": pagination})
}

func buildCodexRefreshHistoryFilters(c *gin.Context) codexRefreshHistoryFilters {
	return codexRefreshHistoryFilters{
		Query:     normalizeAuthFilesFilterValue(c.Query("q")),
		AuthID:    normalizeAuthFilesFilterValue(c.Query("auth_id")),
		AuthIndex: normalizeAuthFilesFilterValue(firstNonEmptyStringPtr(c.Query("auth_index"), c.Query("index"))),
		Email:     normalizeAuthFilesFilterValue(c.Query("email")),
		Result:    normalizeAuthFilesFilterValue(c.Query("result")),
		Trigger:   normalizeAuthFilesFilterValue(c.Query("trigger")),
	}
}

func buildCodexRefreshHistoryEntries(auths []*coreauth.Auth) []gin.H {
	if len(auths) == 0 {
		return nil
	}
	items := make([]gin.H, 0)
	for _, auth := range auths {
		if auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
			continue
		}
		auth.EnsureIndex()
		name := strings.TrimSpace(auth.FileName)
		if name == "" {
			name = strings.TrimSpace(auth.ID)
		}
		email := authEmail(auth)
		history := coreauth.ListRefreshHistory(auth)
		for _, item := range history {
			entry := gin.H{
				"provider":   "codex",
				"auth_id":    strings.TrimSpace(auth.ID),
				"auth_index": strings.TrimSpace(auth.Index),
				"name":       name,
				"email":      email,
				"trigger":    strings.TrimSpace(item.Trigger),
				"result":     strings.TrimSpace(item.Result),
				"message":    strings.TrimSpace(item.Message),
				"at":         item.At,
			}
			if !item.ExpiresAt.IsZero() {
				entry["expires_at"] = item.ExpiresAt
			}
			items = append(items, entry)
		}
	}
	return items
}

func filterCodexRefreshHistoryEntries(items []gin.H, filters codexRefreshHistoryFilters) []gin.H {
	if !hasCodexRefreshHistoryFilters(filters) {
		return items
	}
	filtered := make([]gin.H, 0, len(items))
	for _, item := range items {
		if matchesCodexRefreshHistoryFilters(item, filters) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func hasCodexRefreshHistoryFilters(filters codexRefreshHistoryFilters) bool {
	return filters.Query != "" ||
		filters.AuthID != "" ||
		filters.AuthIndex != "" ||
		filters.Email != "" ||
		filters.Result != "" ||
		filters.Trigger != ""
}

func matchesCodexRefreshHistoryFilters(item gin.H, filters codexRefreshHistoryFilters) bool {
	if filters.AuthID != "" && !authFileStringContains(item, "auth_id", filters.AuthID) {
		return false
	}
	if filters.AuthIndex != "" && !authFileStringContains(item, "auth_index", filters.AuthIndex) {
		return false
	}
	if filters.Email != "" && !authFileStringContains(item, "email", filters.Email) {
		return false
	}
	if filters.Result != "" && !authFileStringContains(item, "result", filters.Result) {
		return false
	}
	if filters.Trigger != "" && !authFileStringContains(item, "trigger", filters.Trigger) {
		return false
	}
	if filters.Query == "" {
		return true
	}
	for _, key := range []string{"auth_id", "auth_index", "name", "email", "result", "trigger", "message"} {
		if authFileStringContains(item, key, filters.Query) {
			return true
		}
	}
	return false
}

func sortCodexRefreshHistoryEntries(items []gin.H, sortByRaw, sortOrderRaw string) {
	sortBy := normalizeCodexRefreshHistorySortBy(sortByRaw)
	desc := normalizeCodexRefreshHistorySortOrder(sortOrderRaw)
	sort.SliceStable(items, func(i, j int) bool {
		cmp := compareCodexRefreshHistoryEntries(items[i], items[j], sortBy)
		if cmp == 0 {
			cmp = compareCodexRefreshHistoryEntries(items[i], items[j], "at")
		}
		if cmp == 0 {
			cmp = compareCodexRefreshHistoryEntries(items[i], items[j], "auth_id")
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func normalizeCodexRefreshHistorySortBy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "at", "created_at":
		return "at"
	case "expires_at", "expiry", "expires":
		return "expires_at"
	case "auth_id", "id":
		return "auth_id"
	case "auth_index", "index":
		return "auth_index"
	case "name":
		return "name"
	case "email":
		return "email"
	case "result":
		return "result"
	case "trigger":
		return "trigger"
	default:
		return "at"
	}
}

func normalizeCodexRefreshHistorySortOrder(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "desc", "descending", "-1":
		return true
	default:
		return false
	}
}

func compareCodexRefreshHistoryEntries(left, right gin.H, sortBy string) int {
	switch sortBy {
	case "at", "expires_at":
		return compareTime(authFileTimeValue(left, sortBy), authFileTimeValue(right, sortBy))
	default:
		return compareStrings(authFileStringValue(left, sortBy), authFileStringValue(right, sortBy))
	}
}

func parseCodexRefreshHistoryLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 20
	}
	limit := parseAuthFilesListLimit(raw)
	if limit <= 0 {
		return 20
	}
	return limit
}

func firstNonEmptyStringPtr(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
