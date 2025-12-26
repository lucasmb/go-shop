package main

import (
	"database/sql"
	"encoding/gob"
	"fmt"
	d "go-shop/internal/data"
	"go-shop/internal/filestore"
	"go-shop/internal/handlers"
	"go-shop/internal/templates"
	"log/slog"
	"os"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"

	// "github.com/mercadopago/sdk-go/v2/config"
	"github.com/stripe/stripe-go/v76"
)

func main() {
	gob.Register(&d.Cart{})
	gob.Register(&d.User{})
	gob.Register(&d.CartItem{})

	err := godotenv.Load()
	if err != nil {
		slog.Error("Error loading .env file")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	db, err := openDB(os.Getenv("DB_DSN"))
	if err != nil {
		logger.Error("cannot connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	// _, err = config.New(os.Getenv("MERCADO_PAGO_ACCESS_TOKEN"))
	// if err != nil {
	// 	logger.Error("failed to configure Mercado Pago", "err", err)
	// }

	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		logger.Error("failed to create template cache", "err", err)
		os.Exit(1)
	}

	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(db)
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Secure = false

	// --- FILESTORE INITIALIZATION ---
	fsConfig := filestore.Config{
		Provider:  os.Getenv("FILESTORE_PROVIDER"),
		LocalPath: os.Getenv("FILESTORE_LOCAL_PATH"),
		BaseURL:   os.Getenv("FILESTORE_BASE_URL"),
		// ... S3 configs ...
	}
	store, err := filestore.New(fsConfig)
	if err != nil {
		logger.Error("failed to create filestore", "err", err)
		os.Exit(1)
	}
	// --- END ---
	app := &application{
		logger:         logger,
		sessionManager: sessionManager,
		handlers: &handlers.Application{
			Logger:         logger,
			Products:       &d.ProductModel{DB: db, Logger: logger},
			Users:          &d.UserModel{DB: db, Logger: logger},
			Orders:         &d.OrderModel{DB: db, Logger: logger},
			Categories:     &d.CategoryModel{DB: db, Logger: logger},
			TemplateCache:  templateCache,
			SessionManager: sessionManager,
			Store:          store, // Inject the filestore
			StripePubKey:   os.Getenv("STRIPE_PUBLISHABLE_KEY"),
		},
	}

	err = app.serve()
	logger.Error("server error", "err", err)
	os.Exit(1)
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	schema, err := os.ReadFile("./schema.sql")
	if err != nil {
		return nil, fmt.Errorf("could not read schema file: %w", err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		return nil, fmt.Errorf("could not apply schema: %w", err)
	}
	return db, nil
}
