package templates

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"go-shop/ui"
)

func add(a, b int) int {
	return a + b
}

func subtract(a, b int) int {
	return a - b
}

// multiply multiplies an int64 by an int, returning an int64.
// It's used to calculate the total price for a cart item.
func multiply(a int64, b int) int64 {
	return a * int64(b)
}

// formatPrice converts cents (int64) to a formatted string like "$12.34".
func formatPrice(price int64) string {
	dollars := price / 100
	cents := price % 100
	return fmt.Sprintf("$%d.%02d", dollars, cents)
}

// dict creates a map from a list of key-value pairs.
// It allows us to construct a data context for a partial on the fly.
func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict expects an even number of arguments")
	}
	d := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		d[key] = values[i+1]
	}
	return d, nil
}

// ------------------------------------
var functions = template.FuncMap{
	"add":         add,
	"subtract":    subtract,
	"formatPrice": formatPrice,
	"itoa":        strconv.Itoa,
	"multiply":    multiply,
	"dict":        dict,
	"import": func(path string) (interface{}, error) { // Add this generic import function
		switch path {
		case "strings":
			return &strings.Builder{}, nil // Return a type from the package to make its functions available
		default:
			return nil, fmt.Errorf("unknown package path: %s", path)
		}
	},
}

func NewTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	// Step 1: Get all the "page" templates (e.g., home.page.html)
	pages, err := fs.Glob(ui.Files, "html/pages/*.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		// Define which files to parse for this page
		files := []string{
			"html/layouts/base.layout.html",
			"html/partials/*.html",
			page, // The page file itself
		}
		if strings.HasPrefix(name, "admin_") {
			files[0] = "html/layouts/admin.layout.html"
		}

		// Create the template set by parsing from the EMBEDDED filesystem (ui.Files)
		ts, err := template.New(name).Funcs(functions).ParseFS(ui.Files, files...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	// Step 2: Cache partials for HTMX swaps.
	// This part is for rendering partials on their own.
	partials, err := fs.Glob(ui.Files, "html/partials/*.html")
	if err != nil {
		return nil, err
	}

	for _, partial := range partials {
		name := filepath.Base(partial)

		// Create a set for each partial that includes ALL other partials,
		// allowing partials to call other partials.
		ts, err := template.New(name).Funcs(functions).ParseFS(ui.Files, "html/partials/*.html")
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	return cache, nil
}
