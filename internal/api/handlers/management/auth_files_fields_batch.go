package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type patchAuthFilesFieldsBatchRequest struct {
	Names       []string          `json:"names"`
	Prefix      *string           `json:"prefix"`
	Headers     map[string]string `json:"headers"`
	Priority    *int              `json:"priority"`
	Note        *string           `json:"note"`
	DryRun      bool              `json:"dry_run"`
	StopOnError bool              `json:"stop_on_error"`
}

type patchAuthFilesFieldsBatchSummary struct {
	Total     int `json:"total"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

type patchAuthFilesFieldsBatchResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Changed bool   `json:"changed"`
	Before  gin.H  `json:"before"`
	After   gin.H  `json:"after"`
	Error   string `json:"error,omitempty"`
}

type patchAuthFilesFieldsBatchResponse struct {
	Status  string                            `json:"status"`
	DryRun  bool                              `json:"dry_run"`
	Summary patchAuthFilesFieldsBatchSummary  `json:"summary"`
	Results []patchAuthFilesFieldsBatchResult `json:"results"`
}

func (req patchAuthFilesFieldsBatchRequest) hasEditableFields() bool {
	return req.Prefix != nil || req.Priority != nil || req.Note != nil || len(req.Headers) > 0
}

// PatchAuthFilesFieldsBatch batch-updates editable auth file fields except proxy_url.
// PATCH /v0/management/auth-files/fields/batch
func (h *Handler) PatchAuthFilesFieldsBatch(c *gin.Context) {
	if h == nil || h.cfg == nil || strings.TrimSpace(h.cfg.AuthDir) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth directory is not configured"})
		return
	}

	var req patchAuthFilesFieldsBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	names := normalizeBatchAuthFileNames(req.Names)
	if len(names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "names is required"})
		return
	}
	if len(names) > maxBatchAuthFileUpdates {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many names, max is %d", maxBatchAuthFileUpdates)})
		return
	}
	if !req.hasEditableFields() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	resp := patchAuthFilesFieldsBatchResponse{
		DryRun: req.DryRun,
		Summary: patchAuthFilesFieldsBatchSummary{
			Total: len(names),
		},
		Results: make([]patchAuthFilesFieldsBatchResult, 0, len(names)),
	}

	ctx := c.Request.Context()
	stopped := false
	for _, name := range names {
		result := patchAuthFilesFieldsBatchResult{Name: name}
		if stopped {
			result.Status = "skipped"
			resp.Summary.Skipped++
			resp.Results = append(resp.Results, result)
			continue
		}

		before, after, changed, err := h.applyFieldsPatchToAuthFile(ctx, name, req)
		result.Before = before
		result.After = after
		result.Changed = changed
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			resp.Summary.Failed++
			if req.StopOnError {
				stopped = true
			}
			resp.Results = append(resp.Results, result)
			continue
		}

		if changed {
			if req.DryRun {
				result.Status = "would_update"
			} else {
				result.Status = "updated"
			}
			resp.Summary.Updated++
		} else {
			result.Status = "unchanged"
			resp.Summary.Unchanged++
		}
		resp.Results = append(resp.Results, result)
	}

	if req.DryRun {
		resp.Status = "dry_run"
	} else if resp.Summary.Failed > 0 {
		resp.Status = "partial"
	} else {
		resp.Status = "ok"
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) applyFieldsPatchToAuthFile(ctx context.Context, name string, req patchAuthFilesFieldsBatchRequest) (gin.H, gin.H, bool, error) {
	if h == nil || h.cfg == nil || strings.TrimSpace(h.cfg.AuthDir) == "" {
		return nil, nil, false, fmt.Errorf("auth directory is not configured")
	}
	if err := validateAuthFileName(name); err != nil {
		return nil, nil, false, err
	}

	full := filepath.Join(h.cfg.AuthDir, filepath.Base(name))
	if !filepath.IsAbs(full) {
		if abs, errAbs := filepath.Abs(full); errAbs == nil {
			full = abs
		}
	}

	data, errRead := os.ReadFile(full)
	if errRead != nil {
		if os.IsNotExist(errRead) {
			return nil, nil, false, fmt.Errorf("file not found")
		}
		return nil, nil, false, fmt.Errorf("failed to read file: %w", errRead)
	}

	metadata := make(map[string]any)
	if errUnmarshal := json.Unmarshal(data, &metadata); errUnmarshal != nil {
		return nil, nil, false, fmt.Errorf("invalid auth file format: %w", errUnmarshal)
	}

	before := snapshotBatchEditableFields(metadata)
	nextMetadata := cloneMetadataMap(metadata)

	if req.Prefix != nil {
		prefixValue, errPrefix := normalizeAuthPrefixInput(*req.Prefix)
		if errPrefix != nil {
			return before, before, false, errPrefix
		}
		if prefixValue == "" {
			delete(nextMetadata, "prefix")
		} else {
			nextMetadata["prefix"] = prefixValue
		}
	}

	if len(req.Headers) > 0 {
		nextHeaders := make(map[string]string)
		for key, value := range coreauth.ExtractCustomHeadersFromMetadata(nextMetadata) {
			nextHeaders[key] = value
		}
		for key, value := range req.Headers {
			headerName := strings.TrimSpace(key)
			if headerName == "" {
				continue
			}
			headerValue := strings.TrimSpace(value)
			if headerValue == "" {
				delete(nextHeaders, headerName)
				continue
			}
			nextHeaders[headerName] = headerValue
		}
		if len(nextHeaders) == 0 {
			delete(nextMetadata, "headers")
		} else {
			metaHeaders := make(map[string]any, len(nextHeaders))
			for key, value := range nextHeaders {
				metaHeaders[key] = value
			}
			nextMetadata["headers"] = metaHeaders
		}
	}

	if req.Priority != nil {
		if *req.Priority == 0 {
			delete(nextMetadata, "priority")
		} else {
			nextMetadata["priority"] = *req.Priority
		}
	}

	if req.Note != nil {
		noteValue := strings.TrimSpace(*req.Note)
		if noteValue == "" {
			delete(nextMetadata, "note")
		} else {
			nextMetadata["note"] = noteValue
		}
	}

	after := snapshotBatchEditableFields(nextMetadata)
	changed := !reflect.DeepEqual(before, after)
	if !changed || req.DryRun {
		return before, after, changed, nil
	}

	newData, errMarshal := json.MarshalIndent(nextMetadata, "", "  ")
	if errMarshal != nil {
		return before, after, changed, fmt.Errorf("failed to serialize file: %w", errMarshal)
	}
	if errWrite := os.WriteFile(full, newData, 0o600); errWrite != nil {
		return before, after, changed, fmt.Errorf("failed to write file: %w", errWrite)
	}
	if errReload := h.reloadAuthFile(ctx, full, newData); errReload != nil {
		return before, after, changed, fmt.Errorf("failed to reload auth file: %w", errReload)
	}
	return before, after, changed, nil
}

func snapshotBatchEditableFields(metadata map[string]any) gin.H {
	snapshot := gin.H{}

	if prefix := normalizeAuthPrefixMetadata(readStringFromMetadata(metadata, "prefix")); prefix != "" {
		snapshot["prefix"] = prefix
	}

	headers := coreauth.ExtractCustomHeadersFromMetadata(metadata)
	if len(headers) > 0 {
		copied := make(map[string]string, len(headers))
		for key, value := range headers {
			copied[key] = value
		}
		snapshot["headers"] = copied
	}

	if priority, ok := parsePriorityValue(metadata["priority"]); ok && priority != 0 {
		snapshot["priority"] = priority
	}

	if note := strings.TrimSpace(readStringFromMetadata(metadata, "note")); note != "" {
		snapshot["note"] = note
	}

	return snapshot
}
