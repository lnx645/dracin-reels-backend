// Command server is the Dracin Reels API server (API only, no HTML).
package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/lnx645/dracin-reels-backend/internal/application"
	"github.com/lnx645/dracin-reels-backend/internal/infrastructure/database"
	"github.com/lnx645/dracin-reels-backend/internal/infrastructure/postgres"
	presentation "github.com/lnx645/dracin-reels-backend/internal/presentation/http"
)

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// loadDotEnv loads a simple KEY=VALUE .env file if present, without
// overriding variables already set in the environment.
func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}

func main() {
	loadDotEnv(".env")
	loadDotEnv("/opt/dracin-reels-backend/.env")

	port := getenv("PORT", "8100")
	dsn := getenv("DATABASE_URL", "")
	if dsn == "" {
		dsn = "postgres://" + getenv("DB_USER", "postgres") + ":" + getenv("DB_PASSWORD", "") +
			"@" + getenv("DB_HOST", "127.0.0.1") + ":" + getenv("DB_PORT", "5432") +
			"/" + getenv("DB_NAME", "dracin_reels") + "?sslmode=disable"
	}
	jwtSecret := getenv("JWT_SECRET", "")
	if jwtSecret == "" {
		log.Println("WARNING: JWT_SECRET is empty, using an insecure dev default. Set JWT_SECRET in production.")
		jwtSecret = "dev-only-change-me"
	}
	mediaDir := getenv("MEDIA_DIR", "media")

	db, err := database.Open(dsn)
	if err != nil {
		log.Fatalf("database connect: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if getenv("SEED_DEMO", "true") == "true" {
		seeded, err := database.SeedIfEmpty(db)
		if err != nil {
			log.Fatalf("seed: %v", err)
		}
		if seeded {
			log.Println("demo seed inserted (admin + viewer demo, Qin series)")
		}
	}

	store := postgres.NewStore(db)
	h := &presentation.Handlers{
		Auth:     application.NewAuthService(store, jwtSecret),
		Catalog:  application.NewCatalogService(store),
		Wallet:   application.NewWalletService(store),
		Social:   application.NewSocialService(store),
		MediaDir: mediaDir,
	}

	log.Printf("dracin-reels-backend listening on :%s (media dir: %s)", port, mediaDir)
	if err := http.ListenAndServe(":"+port, h.Router()); err != nil {
		log.Fatal(err)
	}
}
