package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/config"
	"github.com/sathishkumar-nce/amz-orders/internal/db"
	"github.com/sathishkumar-nce/amz-orders/internal/handlers"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/baselinker"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/delhivery"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/googlesheets"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
	"github.com/sathishkumar-nce/amz-orders/internal/router"
	"github.com/sathishkumar-nce/amz-orders/internal/scheduler"
	"github.com/sathishkumar-nce/amz-orders/internal/service"
	"github.com/sathishkumar-nce/amz-orders/internal/utils"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Load SKU mapper from CSV file
	skuMapper := utils.GetSKUMapper()
	skuCSVPath := os.Getenv("SKU_CSV_PATH")
	if skuCSVPath == "" {
		skuCSVPath = "./SKU_V8.csv" // Default path
	}

	if err := skuMapper.LoadFromCSV(skuCSVPath); err != nil {
		log.Printf("⚠️  Warning: Failed to load SKU mapping CSV: %v", err)
		log.Printf("    Dimension auto-population will be disabled")
	} else {
		log.Printf("✅ SKU mapper loaded successfully: %d SKUs", skuMapper.GetDataCount())
	}
	// Initialize database connection
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize BaseLinker client
	blClient := baselinker.NewClient(cfg.BaseLinkerAPIURL, cfg.BaseLinkerToken)

	// Initialize repositories
	orderRepo := repository.NewOrderRepository(pool)
	directOrderRepo := repository.NewDirectOrderRepository(pool)
	priorityRuleRepo := repository.NewPriorityRuleRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	shippingDateFilterRepo := repository.NewShippingDateFilterRepository(pool)
	rowHighlightRuleRepo := repository.NewAmazonRowHighlightRuleRepository(pool)

	if err := userRepo.EnsureDefaultAdmin(
		ctx,
		cfg.DefaultAdminUsername,
		cfg.DefaultAdminPassword,
		cfg.DefaultAdminEmail,
	); err != nil {
		log.Fatalf("Failed to ensure default admin user: %v", err)
	}

	// Initialize Google Sheets client (optional)
	sheetsCredFile := os.Getenv("GOOGLE_SHEETS_CREDENTIALS")
	if sheetsCredFile == "" {
		sheetsCredFile = "./spreadsheet.json" // Default path
	}
	sheetsID := os.Getenv("GOOGLE_SHEETS_ID")
	if sheetsID == "" {
		sheetsID = "1Lzk8ceP5GZ8ePRQVjVX6Nqt676N9-6i_Drcry5emfjE" // Default sheet ID
	}

	sheetsClient, err := googlesheets.NewClient(ctx, sheetsCredFile, sheetsID)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to initialize Google Sheets client: %v", err)
		log.Printf("    Orders will not be synced to Google Sheets")
	} else {
		orderRepo.SetGoogleSheetsClient(sheetsClient)
		log.Printf("✅ Google Sheets integration enabled (Sheet ID: %s)", sheetsID)
	}

	// Initialize services and handlers
	orderService := service.NewOrderService(orderRepo, priorityRuleRepo, blClient)
	priorityRuleService := service.NewPriorityRuleService(priorityRuleRepo)
	shippingDateFilterService := service.NewShippingDateFilterService(shippingDateFilterRepo)
	rowHighlightRuleService := service.NewAmazonRowHighlightRuleService(rowHighlightRuleRepo)
	dbBackupService := service.NewDBBackupService(
		cfg.DatabaseURL,
		cfg.PGDumpPath,
		cfg.DBBackupLocalDir,
	)
	orderHandler := handlers.NewOrderHandler(orderService)
	authHandler := handlers.NewAuthHandler(userRepo, cfg)
	skuHandler := handlers.NewSKUHandler(skuCSVPath)
	priorityRuleHandler := handlers.NewPriorityRuleHandler(priorityRuleService, orderService)
	shippingDateFilterHandler := handlers.NewShippingDateFilterHandler(shippingDateFilterService)
	rowHighlightRuleHandler := handlers.NewAmazonRowHighlightRuleHandler(rowHighlightRuleService)
	dbBackupHandler := handlers.NewDBBackupHandler(dbBackupService)
	delhiveryClient := delhivery.NewClient(delhivery.Config{
		BaseURL:               cfg.DelhiveryAPIBaseURL,
		APIToken:              cfg.DelhiveryAPIToken,
		ClientName:            cfg.DelhiveryClientName,
		DefaultPickupLocation: cfg.DelhiveryDefaultPickupLocation,
		SellerName:            cfg.DelhiverySellerName,
		SellerAddress:         cfg.DelhiverySellerAddress,
		SellerGSTIN:           cfg.DelhiverySellerGSTIN,
		ClientGSTIN:           cfg.DelhiveryClientGSTIN,
	})
	directOrderHandler := handlers.NewDirectOrderHandler(directOrderRepo, delhiveryClient)

	// Setup router
	r := router.SetupRouter(orderHandler, authHandler, skuHandler, priorityRuleHandler, dbBackupHandler, directOrderHandler, shippingDateFilterHandler, rowHighlightRuleHandler, cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Initialize and start the order sync scheduler (configurable interval)
	syncScheduler := scheduler.NewOrderSyncScheduler(orderService, cfg.SyncIntervalMinutes)
	syncScheduler.Start(ctx)
	log.Printf("⏰ Order sync scheduler started (interval: %d minutes)", cfg.SyncIntervalMinutes)

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Server starting on port %s", cfg.AppPort)
		log.Printf("🔐 JWT authentication enabled")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop the scheduler
	syncScheduler.Stop()

	// Graceful shutdown with 10 second timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
