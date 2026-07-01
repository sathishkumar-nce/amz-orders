package router

import (
	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/config"
	"github.com/sathishkumar-nce/amz-orders/internal/handlers"
	"github.com/sathishkumar-nce/amz-orders/internal/middleware"
)

func SetupRouter(orderHandler *handlers.OrderHandler, authHandler *handlers.AuthHandler, skuHandler *handlers.SKUHandler, priorityRuleHandler *handlers.PriorityRuleHandler, dbBackupHandler *handlers.DBBackupHandler, directOrderHandler *handlers.DirectOrderHandler, shippingDateFilterHandler *handlers.ShippingDateFilterHandler, amazonRowHighlightRuleHandler *handlers.AmazonRowHighlightRuleHandler, interaktSettingsHandler *handlers.InteraktSettingsHandler, cfg *config.Config) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.CORS())

	// Health check (public)
	router.GET("/health", orderHandler.Health)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
		}

		// Protected routes - require JWT authentication
		protected := v1.Group("")
		protected.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			// Auth - Get current user
			protected.GET("/auth/me", authHandler.Me)
			protected.POST("/auth/change-password", authHandler.ChangePassword)
			protected.GET("/users", authHandler.ListUsers)
			protected.POST("/users", authHandler.AdminCreateUser)
			protected.DELETE("/users/:user_id", authHandler.DeleteUser)
			protected.GET("/dashboard/executive", orderHandler.GetExecutiveDashboard)
			protected.GET("/dashboard/returns", orderHandler.GetReturnsDashboard)
			protected.GET("/dashboard/safety-claims", orderHandler.GetSafetyClaimsDashboard)

			// Orders
			orders := protected.Group("/orders")
			{
				orders.POST("/import", orderHandler.ImportFromBaseLinker)
				orders.POST("/import-sample", orderHandler.ImportFromSampleFile)
				orders.GET("", orderHandler.ListOrders)
				orders.GET("/analytics/dashboard", orderHandler.GetDashboardAnalytics)
				orders.GET("/analytics/repeat-customers", orderHandler.GetRepeatOrderCustomers)
				orders.GET("/:amazon_order_id", orderHandler.GetOrder)
				orders.PATCH("/:amazon_order_id/manual", orderHandler.UpdateManualFields)
				orders.PATCH("/:amazon_order_id/products/:order_product_id/manual", orderHandler.UpdateProductManualFields)
			}

			// Issues
			protected.GET("/issues", orderHandler.ListIssues)

			// Returns
			protected.GET("/returns", orderHandler.ListReturns)
			protected.GET("/returns/analytics/repeat-customers", orderHandler.GetRepeatReturnCustomers)

			// Safety Claims
			protected.GET("/safety-claims", orderHandler.ListSafetyClaims)

			// SKU Mapper management
			sku := protected.Group("/sku-mapper")
			{
				sku.GET("", skuHandler.GetSKUFileInfo)
				sku.GET("/download", skuHandler.DownloadSKUFile)
				sku.POST("/upload", skuHandler.UpdateSKUFile)
				sku.POST("/parse-schedule-weights", skuHandler.ParseScheduleWeights)
			}

			priorityRules := protected.Group("/order-priority-rules")
			{
				priorityRules.GET("", priorityRuleHandler.List)
				priorityRules.PUT("", priorityRuleHandler.UpdateAll)
			}

			dbBackups := protected.Group("/db-backups")
			{
				dbBackups.GET("/status", dbBackupHandler.GetStatus)
				dbBackups.POST("/run", dbBackupHandler.RunBackup)
				dbBackups.GET("/download/:file_name", dbBackupHandler.DownloadBackup)
			}

			shippingDateFilters := protected.Group("/shipping-date-filters")
			{
				shippingDateFilters.GET("", shippingDateFilterHandler.List)
				shippingDateFilters.PUT("", shippingDateFilterHandler.UpdateAll)
				shippingDateFilters.PUT("/active", shippingDateFilterHandler.SetActive)
				shippingDateFilters.POST("/reset", shippingDateFilterHandler.ResetDefaults)
			}

			rowHighlightRules := protected.Group("/amazon-row-highlight-rules")
			{
				rowHighlightRules.GET("", amazonRowHighlightRuleHandler.List)
				rowHighlightRules.PUT("", amazonRowHighlightRuleHandler.UpdateAll)
				rowHighlightRules.POST("/reset", amazonRowHighlightRuleHandler.ResetDefaults)
			}

			interaktSettings := protected.Group("/interakt-settings")
			{
				interaktSettings.GET("", interaktSettingsHandler.Get)
				interaktSettings.PUT("", interaktSettingsHandler.Update)
			}

			directOrders := protected.Group("/direct-orders")
			{
				directOrders.GET("", directOrderHandler.ListDirectOrders)
				directOrders.GET("/search", directOrderHandler.SearchDirectOrders)
				directOrders.GET("/dashboard/executive", directOrderHandler.GetExecutiveDashboard)
				directOrders.GET("/next-order-id", directOrderHandler.GetNextDirectOrderID)
				directOrders.GET("/delhivery/pincode-lookup", directOrderHandler.LookupDelhiveryPincode)
				directOrders.GET("/export", directOrderHandler.ExportDirectOrdersCSV)
				directOrders.POST("", directOrderHandler.CreateDirectOrder)
				directOrders.POST("/delhivery/forward-orders/bulk", directOrderHandler.CreateDelhiveryForwardOrdersBulk)
				directOrders.GET("/:order_id", directOrderHandler.GetDirectOrder)
				directOrders.PATCH("/:order_id", directOrderHandler.UpdateDirectOrder)
				directOrders.DELETE("/:order_id", directOrderHandler.DeleteDirectOrder)
				directOrders.POST("/:order_id/delhivery/forward-order", directOrderHandler.CreateDelhiveryForwardOrder)
			}

		}
	}

	return router
}
