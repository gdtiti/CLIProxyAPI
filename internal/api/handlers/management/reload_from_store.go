package management

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	storepkg "github.com/router-for-me/CLIProxyAPI/v6/internal/store"
)

type configStoreReloader interface {
	ReloadConfigFromStore(ctx context.Context) (*storepkg.ConfigReloadResult, error)
}

type authFilesStoreReloader interface {
	ReloadAuthFilesFromStore(ctx context.Context) (*storepkg.AuthReloadResult, error)
}

func (h *Handler) PostReloadConfigFromStore(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler_not_initialized"})
		return
	}

	reloader, ok := h.tokenStore.(configStoreReloader)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "reload_not_supported",
			"message": "current token store does not support reloading config from store",
		})
		return
	}
	if h.reloadConfigHook == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "runtime_reload_unavailable",
			"message": "runtime config reload hook is not available",
		})
		return
	}

	result, err := reloader.ReloadConfigFromStore(c.Request.Context())
	if err != nil {
		if errors.Is(err, storepkg.ErrConfigRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "store_record_not_found",
				"message": "config record not found in store",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "store_reload_failed",
			"message": err.Error(),
		})
		return
	}

	if err = h.reloadConfigHook(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "runtime_reload_failed",
			"message": err.Error(),
			"store":   result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"source":  "store",
		"target":  "config",
		"store":   result.Store,
		"changed": result.Changed,
	})
}

func (h *Handler) PostReloadAuthFilesFromStore(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler_not_initialized"})
		return
	}

	reloader, ok := h.tokenStore.(authFilesStoreReloader)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "reload_not_supported",
			"message": "current token store does not support reloading auth files from store",
		})
		return
	}
	if h.reloadAuthFilesHook == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "runtime_reload_unavailable",
			"message": "runtime auth reload hook is not available",
		})
		return
	}

	result, err := reloader.ReloadAuthFilesFromStore(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "store_reload_failed",
			"message": err.Error(),
		})
		return
	}

	if err = h.reloadAuthFilesHook(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "runtime_reload_failed",
			"message": err.Error(),
			"store":   result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"source":    "store",
		"target":    "auth-files",
		"store":     result.Store,
		"total":     result.Total,
		"written":   result.Written,
		"removed":   result.Removed,
		"unchanged": result.Unchanged,
	})
}
