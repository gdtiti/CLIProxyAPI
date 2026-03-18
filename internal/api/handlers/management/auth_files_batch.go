package management

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type batchAuthUploadItem struct {
	Name string
	Data []byte
}

// UploadAuthFilesBatch supports multi-file multipart uploads and zip archives.
func (h *Handler) UploadAuthFilesBatch(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart form is required"})
		return
	}

	headers := collectBatchUploadHeaders(form.File["files"], form.File["file"])
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no auth files uploaded"})
		return
	}
	if len(headers) > maxBatchAuthFileUpdates {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files, max is %d", maxBatchAuthFileUpdates)})
		return
	}

	items, err := h.collectBatchUploadItems(headers)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid .json auth files found"})
		return
	}
	if len(items) > maxBatchAuthFileUpdates {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files after expanding archives, max is %d", maxBatchAuthFileUpdates)})
		return
	}

	ctx := c.Request.Context()
	uploaded := make([]string, 0, len(items))
	for _, item := range items {
		if err := h.persistUploadedAuthFile(ctx, item.Name, item.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":          err.Error(),
				"uploaded":       len(uploaded),
				"uploaded_names": uploaded,
				"failed_name":    item.Name,
			})
			return
		}
		uploaded = append(uploaded, item.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"uploaded":       len(uploaded),
		"uploaded_names": uploaded,
	})
}

func collectBatchUploadHeaders(groups ...[]*multipart.FileHeader) []*multipart.FileHeader {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	headers := make([]*multipart.FileHeader, 0, total)
	for _, group := range groups {
		for _, header := range group {
			if header != nil {
				headers = append(headers, header)
			}
		}
	}
	return headers
}

func (h *Handler) collectBatchUploadItems(headers []*multipart.FileHeader) ([]batchAuthUploadItem, error) {
	items := make([]batchAuthUploadItem, 0, len(headers))
	seen := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		extracted, err := h.readBatchUploadHeader(header)
		if err != nil {
			return nil, err
		}
		for _, item := range extracted {
			name := filepath.Base(strings.TrimSpace(item.Name))
			if name == "" {
				return nil, fmt.Errorf("invalid uploaded file name")
			}
			if _, exists := seen[name]; exists {
				return nil, fmt.Errorf("duplicate auth file name: %s", name)
			}
			seen[name] = struct{}{}
			items = append(items, batchAuthUploadItem{Name: name, Data: item.Data})
		}
	}
	return items, nil
}

func (h *Handler) readBatchUploadHeader(header *multipart.FileHeader) ([]batchAuthUploadItem, error) {
	if header == nil {
		return nil, nil
	}
	name := filepath.Base(strings.TrimSpace(header.Filename))
	if name == "" {
		return nil, fmt.Errorf("invalid uploaded file name")
	}
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file %s: %w", name, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploaded file %s: %w", name, err)
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		return []batchAuthUploadItem{{Name: name, Data: data}}, nil
	case ".zip":
		return readBatchZipItems(name, data)
	default:
		return nil, fmt.Errorf("unsupported upload %s: only .json or .zip is allowed", name)
	}
}

func readBatchZipItems(name string, data []byte) ([]batchAuthUploadItem, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip %s: %w", name, err)
	}
	items := make([]batchAuthUploadItem, 0, len(reader.File))
	for _, file := range reader.File {
		if file == nil || file.FileInfo().IsDir() {
			continue
		}
		entryName := filepath.Base(strings.TrimSpace(file.Name))
		if !strings.HasSuffix(strings.ToLower(entryName), ".json") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open zip entry %s: %w", file.Name, err)
		}
		entryData, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read zip entry %s: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("failed to close zip entry %s: %w", file.Name, closeErr)
		}
		items = append(items, batchAuthUploadItem{Name: entryName, Data: entryData})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("zip %s does not contain any .json auth files", name)
	}
	return items, nil
}
