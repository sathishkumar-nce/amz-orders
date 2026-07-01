package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort                        string
	DatabaseURL                    string
	BaseLinkerAPIURL               string
	BaseLinkerToken                string
	DelhiveryAPIBaseURL            string
	DelhiveryAPIToken              string
	DelhiveryClientName            string
	DelhiveryDefaultPickupLocation string
	DelhiverySellerName            string
	DelhiverySellerAddress         string
	DelhiverySellerGSTIN           string
	DelhiveryClientGSTIN           string
	SyncIntervalMinutes            int
	JWTSecret                      string
	JWTExpirationHours             int
	DefaultAdminUsername           string
	DefaultAdminPassword           string
	DefaultAdminEmail              string
	PGDumpPath                     string
	DBBackupLocalDir               string
	InteraktEnabled                bool
	InteraktAPIBaseURL             string
	InteraktAPIKey                 string
	InteraktMode                   string
	InteraktTestNumber             string
	InteraktTemplateName           string
}

func Load() *Config {
	loadOptionalEnv(".env")
	loadOptionalEnv(".env.docker")
	loadOptionalEnv(filepath.Join("..", ".env"))
	loadOptionalEnv(filepath.Join("..", ".env.docker"))

	syncInterval := 5 // Default 5 minutes
	if val := os.Getenv("SYNC_INTERVAL_MINUTES"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			syncInterval = parsed
		}
	}

	jwtExpiration := 24 // Default 24 hours
	if val := os.Getenv("JWT_EXPIRATION_HOURS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			jwtExpiration = parsed
		}
	}

	interaktEnabled := false
	if val := os.Getenv("INTERAKT_ENABLED"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			interaktEnabled = parsed
		}
	}

	return &Config{
		AppPort:                        getEnv("APP_PORT", "8080"),
		DatabaseURL:                    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/amz_orders?sslmode=disable"),
		BaseLinkerAPIURL:               getEnv("BASELINKER_API_URL", "https://api.baselinker.com/connector.php"),
		BaseLinkerToken:                getEnv("BASELINKER_TOKEN", ""),
		DelhiveryAPIBaseURL:            "https://track.delhivery.com",
		DelhiveryAPIToken:              getEnv("DELHIVERY_API_TOKEN", ""),
		DelhiveryClientName:            getEnv("DELHIVERY_CLIENT_NAME", ""),
		DelhiveryDefaultPickupLocation: getEnv("DELHIVERY_DEFAULT_PICKUP_LOCATION", ""),
		DelhiverySellerName:            getEnv("DELHIVERY_SELLER_NAME", ""),
		DelhiverySellerAddress:         getEnv("DELHIVERY_SELLER_ADDRESS", ""),
		DelhiverySellerGSTIN:           getEnv("DELHIVERY_SELLER_GSTIN", ""),
		DelhiveryClientGSTIN:           getEnv("DELHIVERY_CLIENT_GSTIN", ""),
		SyncIntervalMinutes:            syncInterval,
		JWTSecret:                      getEnv("JWT_SECRET", "your-secret-key-change-this"),
		JWTExpirationHours:             jwtExpiration,
		DefaultAdminUsername:           getEnv("DEFAULT_ADMIN_USERNAME", "admin"),
		DefaultAdminPassword:           getEnv("DEFAULT_ADMIN_PASSWORD", "admin123"),
		DefaultAdminEmail:              getEnv("DEFAULT_ADMIN_EMAIL", "admin@example.com"),
		PGDumpPath:                     getEnv("PG_DUMP_PATH", "pg_dump"),
		DBBackupLocalDir:               getEnv("DB_BACKUP_LOCAL_DIR", "./backups"),
		InteraktEnabled:                interaktEnabled,
		InteraktAPIBaseURL:             getEnv("INTERAKT_API_BASE_URL", "https://api.interakt.ai"),
		InteraktAPIKey:                 getEnv("INTERAKT_API_KEY", ""),
		InteraktMode:                   getEnv("INTERAKT_MODE", "prod"), // test or prod
		InteraktTestNumber:             getEnv("INTERAKT_TEST_NUMBER", ""),
		InteraktTemplateName:           getEnv("INTERAKT_TEMPLATE_NAME", "amzmrclearorderconfirmation_v2"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func loadOptionalEnv(path string) {
	if err := godotenv.Load(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Warning: failed to load %s: %v", path, err)
	}
}
