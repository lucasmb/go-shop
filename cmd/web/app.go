package main

import (
	"fmt"
	"go-shop/internal/handlers"
	"go-shop/ui"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
)

type application struct {
	logger         *slog.Logger
	handlers       *handlers.Application
	sessionManager *scs.SessionManager
}

// App routes
func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// filesystem object rooted in the 'static' directory of embedded files.
	staticFS, err := fs.Sub(ui.Files, "static")
	if err != nil {
		panic(err)
	}
	// file server that serves files from this embedded filesystem.
	fileServer := http.FileServer(http.FS(staticFS))

	mux.Handle("/static/", http.StripPrefix("/static", fileServer))
	// Serve uploaded media files
	mediaFS := http.FileServer(http.Dir(os.Getenv("FILESTORE_LOCAL_PATH")))
	mux.Handle("/media/", http.StripPrefix("/media", mediaFS))

	// Middleware chains
	dynamic := handlers.NewChain(app.handlers.PopulateUser)
	protected := dynamic.Append(app.handlers.RequireAuthentication)
	admin := protected.Append(app.handlers.RequireAdmin)

	// Route definitions
	mux.Handle("GET /{$}", dynamic.ThenFunc(app.handlers.Home))
	mux.Handle("GET /product/{id}", dynamic.ThenFunc(app.handlers.ProductDetail))
	mux.Handle("GET /register", dynamic.ThenFunc(app.handlers.ShowRegister))
	mux.Handle("POST /register", dynamic.ThenFunc(app.handlers.DoRegister))
	mux.Handle("GET /login", dynamic.ThenFunc(app.handlers.ShowLogin))
	mux.Handle("POST /login", dynamic.ThenFunc(app.handlers.DoLogin))
	mux.Handle("POST /logout", protected.ThenFunc(app.handlers.DoLogout))
	mux.Handle("GET /cart", dynamic.ThenFunc(app.handlers.ViewCart))
	mux.Handle("GET /my-orders", protected.ThenFunc(app.handlers.ShowMyOrders))
	mux.Handle("GET /my-orders/{id}", protected.ThenFunc(app.handlers.ShowOrderDetail))
	mux.Handle("GET /checkout", protected.ThenFunc(app.handlers.ShowCheckout))
	mux.Handle("GET /order/success", protected.ThenFunc(app.handlers.OrderSuccess))
	mux.Handle("POST /cart/add/{id}", dynamic.ThenFunc(app.handlers.AddToCart))
	mux.Handle("POST /cart/update/{id}", dynamic.ThenFunc(app.handlers.UpdateCartItem))
	mux.Handle("DELETE /cart/item/{id}", dynamic.ThenFunc(app.handlers.RemoveFromCart))
	mux.Handle("POST /checkout/create-payment-intent", protected.ThenFunc(app.handlers.CreatePaymentIntent))
	mux.Handle("POST /checkout/bank-deposit", protected.ThenFunc(app.handlers.CreateBankDepositOrder))

	mux.Handle("GET /admin", admin.ThenFunc(app.handlers.AdminDashboard)) // Dashboard home
	mux.Handle("GET /admin/orders", admin.ThenFunc(app.handlers.AdminShowOrders))
	mux.Handle("POST /admin/order/update-status/{id}", admin.ThenFunc(app.handlers.AdminUpdateOrder))

	// Product CRUD Routes
	mux.Handle("GET /admin/products", admin.ThenFunc(app.handlers.AdminListProducts))
	mux.Handle("GET /admin/products/new", admin.ThenFunc(app.handlers.AdminProductCreate))
	mux.Handle("POST /admin/products", admin.ThenFunc(app.handlers.AdminProductCreate))
	mux.Handle("GET /admin/products/{id}", admin.ThenFunc(app.handlers.AdminProductEdit))
	mux.Handle("PUT /admin/products/{id}", admin.ThenFunc(app.handlers.AdminProductEdit))
	mux.Handle("DELETE /admin/products/{id}", admin.ThenFunc(app.handlers.AdminDeleteProduct))
	mux.Handle("POST /admin/products/form/add-group", admin.ThenFunc(app.handlers.AdminAddVariantGroup))
	mux.Handle("POST /admin/products/form/generate-skus", admin.ThenFunc(app.handlers.AdminGenerateSKUs))

	//  API endpoint for chart data
	mux.Handle("POST /admin/api/upload-image", admin.ThenFunc(app.handlers.APIUploadImage))
	mux.Handle("GET /admin/api/sales-report", admin.ThenFunc(app.handlers.APISalesReport))
	mux.Handle("GET /admin/api/product-sales-report", admin.ThenFunc(app.handlers.APIProductSalesReport))

	// 1. Create the nosurf middleware, wrapping our main router (mux)
	csrfHandler := nosurf.New(mux)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   false, // Set to true in production with HTTPS
	})

	csrfHandler.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.logger.Error("!!!!!! CSRF FAILURE DETECTED !!!!!!", "path", r.URL.Path, "reason", nosurf.Reason(r))
		http.Error(w, nosurf.Reason(r).Error(), http.StatusBadRequest)
	}))

	// 2. Wrap the CSRF handler with our session manager.
	// session must load before CSRF can work.
	finalHandler := app.sessionManager.LoadAndSave(csrfHandler)

	// 3. Return the final, fully wrapped handler.
	return finalHandler
}

// We can also move the server startup logic into a method for clarity.
func (app *application) serve() error {
	srv := &http.Server{
		Addr:     ":4000", // This should come from config
		Handler:  app.routes(),
		ErrorLog: slog.NewLogLogger(app.logger.Handler(), slog.LevelError)}

	app.logger.Info(fmt.Sprintf("Starting server on %s", srv.Addr))
	return srv.ListenAndServe()
}
