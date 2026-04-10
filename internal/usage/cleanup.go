package usage

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// CleanupWorker periodically deletes expired usage data from PG.
type CleanupWorker struct {
	interval      time.Duration // check interval, default 1h
	retainDays    int           // detail retention days, default 30
	aggRetainDays int           // aggregate retention days, default 365
	batchSize     int           // batch delete size, default 1000
	db            *sql.DB
	done          chan struct{}
	wg            sync.WaitGroup
}

// NewCleanupWorker creates a CleanupWorker with the given retention settings.
func NewCleanupWorker(db *sql.DB, retainDays, aggRetainDays int) *CleanupWorker {
	if retainDays <= 0 {
		retainDays = 30
	}
	if aggRetainDays <= 0 {
		aggRetainDays = 365
	}
	return &CleanupWorker{
		interval:      time.Hour,
		retainDays:    retainDays,
		aggRetainDays: aggRetainDays,
		batchSize:     1000,
		db:            db,
		done:          make(chan struct{}),
	}
}

// Start launches the background cleanup goroutine.
func (w *CleanupWorker) Start() {
	w.wg.Add(1)
	go w.run()
	log.Infof("CleanupWorker started: detail_retain=%dd, agg_retain=%dd", w.retainDays, w.aggRetainDays)
}

// Stop gracefully shuts down the cleanup worker.
func (w *CleanupWorker) Stop() {
	close(w.done)
	w.wg.Wait()
	log.Info("CleanupWorker stopped")
}

func (w *CleanupWorker) run() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run once immediately on start
	w.cleanup()

	for {
		select {
		case <-ticker.C:
			w.cleanup()
		case <-w.done:
			return
		}
	}
}

func (w *CleanupWorker) cleanup() {
	// Clean expired usage_details
	detailBefore := time.Now().AddDate(0, 0, -w.retainDays)
	if err := w.deleteBatch("usage_details", "created_at", detailBefore); err != nil {
		log.Errorf("CleanupWorker: failed to clean usage_details: %v", err)
	} else {
		log.Debugf("CleanupWorker: cleaned usage_details older than %s", detailBefore.Format(time.RFC3339))
	}

	// Clean expired usage_aggregates
	aggBefore := time.Now().AddDate(0, 0, -w.aggRetainDays)
	if err := w.deleteBatch("usage_aggregates", "bucket_hour", aggBefore); err != nil {
		log.Errorf("CleanupWorker: failed to clean usage_aggregates: %v", err)
	} else {
		log.Debugf("CleanupWorker: cleaned usage_aggregates older than %s", aggBefore.Format(time.RFC3339))
	}
}

// deleteBatch deletes rows in batches to avoid long transactions.
func (w *CleanupWorker) deleteBatch(table, column string, before time.Time) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE ctid IN (SELECT ctid FROM %s WHERE %s < $1 LIMIT $2)",
		table, table, column,
	)
	for {
		result, err := w.db.Exec(query, before, w.batchSize)
		if err != nil {
			return fmt.Errorf("cleanup %s: %w", table, err)
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			break
		}
		log.Debugf("CleanupWorker: deleted %d rows from %s", affected, table)
	}
	return nil
}
