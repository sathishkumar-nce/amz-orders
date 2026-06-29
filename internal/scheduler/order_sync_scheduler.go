package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/service"
)

type OrderSyncScheduler struct {
	orderService *service.OrderService
	interval     time.Duration
	stopChan     chan bool
	isRunning    bool
	isFirstRun   bool
	mu           sync.Mutex
	syncActive   bool
}

func NewOrderSyncScheduler(orderService *service.OrderService, intervalMinutes int) *OrderSyncScheduler {
	return &OrderSyncScheduler{
		orderService: orderService,
		interval:     time.Duration(intervalMinutes) * time.Minute,
		stopChan:     make(chan bool),
		isRunning:    false,
		isFirstRun:   true, // First run flag
	}
}

// Start begins the scheduled sync process
func (s *OrderSyncScheduler) Start(ctx context.Context) {
	if s.isRunning {
		log.Println("Scheduler already running")
		return
	}

	s.isRunning = true
	log.Printf("Starting order sync scheduler (interval: %v)", s.interval)

	// Run immediately on start
	go s.syncOrders(ctx)

	// Then run on schedule
	ticker := time.NewTicker(s.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.syncOrders(ctx)
			case <-s.stopChan:
				ticker.Stop()
				log.Println("Order sync scheduler stopped")
				return
			case <-ctx.Done():
				ticker.Stop()
				log.Println("Order sync scheduler stopped due to context cancellation")
				return
			}
		}
	}()
}

// Stop halts the scheduler
func (s *OrderSyncScheduler) Stop() {
	if !s.isRunning {
		return
	}
	s.stopChan <- true
	s.isRunning = false
}

// syncOrders performs the actual sync operation
func (s *OrderSyncScheduler) syncOrders(ctx context.Context) {
	s.mu.Lock()
	if s.syncActive {
		s.mu.Unlock()
		log.Println("Skipping scheduled order sync because a previous sync is still running")
		return
	}
	s.syncActive = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.syncActive = false
		s.mu.Unlock()
	}()

	log.Println("Starting scheduled order sync from BaseLinker...")
	startTime := time.Now()

	var dateFrom int64
	var fetchWindow string

	if s.isFirstRun {
		// First run: fetch orders from the last 6 hours
		dateFrom = time.Now().Add(-6 * time.Hour).Unix()
		fetchWindow = "6 hours"
		s.isFirstRun = false

		log.Println("🚀 FIRST RUN: Fetching confirmed orders from the last 6 hours...")
	} else {
		// Subsequent runs: fetch orders from the last 3 hours
		dateFrom = time.Now().Add(-3 * time.Hour).Unix()
		fetchWindow = "3 hours"
	}

	totalFetched, totalOrders, totalProducts, err := s.orderService.ImportFromBaseLinker(ctx, dateFrom)
	if err != nil {
		log.Printf("ERROR: Scheduled sync failed: %v", err)
		return
	}

	duration := time.Since(startTime)

	log.Printf(
		"✓ Scheduled sync completed in %v (window: %s): fetched=%d, orders=%d, products=%d",
		duration,
		fetchWindow,
		totalFetched,
		totalOrders,
		totalProducts,
	)
}

// SyncNow triggers an immediate sync (useful for manual triggers)
func (s *OrderSyncScheduler) SyncNow(ctx context.Context) {
	log.Println("Manual sync triggered")
	go s.syncOrders(ctx)
}
