package main

import (
	"testing"

	"github.com/alexedwards/scs/v2"

	d "go-shop/internal/data"
	"go-shop/internal/handlers"
	"go-shop/internal/templates"
	"go-shop/internal/testutils"
)

// newTestApplication returns a fully functional application instance.
func newTestApplication(t *testing.T) *application {
	logger := testutils.NewTestLogger()
	db := testutils.NewTestDB(t) // CREATE A REAL IN-MEMORY DB

	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("failed to create template cache: %v", err)
	}

	sessionManager := scs.New()
	// No store needed for this test, default is in-memory

	return &application{
		logger:         logger,
		sessionManager: sessionManager,
		handlers: &handlers.Application{
			Logger: logger,

			// Initialize ALL models with the real test database connection.
			Products:       &d.ProductModel{DB: db, Logger: logger},
			Categories:     &d.CategoryModel{DB: db, Logger: logger},
			Users:          &d.UserModel{DB: db, Logger: logger},
			Orders:         &d.OrderModel{DB: db, Logger: logger},
			TemplateCache:  templateCache,
			SessionManager: sessionManager,
		},
	}
}
