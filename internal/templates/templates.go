package templates

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strconv"

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

var functions = template.FuncMap{
	"add":         add,
	"subtract":    subtract,
	"formatPrice": formatPrice,
	"itoa":        strconv.Itoa, // Helper to convert int to string in templates
	"multiply":    multiply,
}

func NewTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	// Step 1: Cache pages
	pages, err := fs.Glob(ui.Files, "html/pages/*.html")
	if err != nil {
		return nil, err
	}
	for _, page := range pages {
		name := filepath.Base(page)
		ts, err := template.New(name).Funcs(functions).ParseFS(ui.Files,
			"html/layouts/base.layout.html",
			"html/partials/*.html", // This will now correctly ignore the deleted toast.partial.html
		)
		if err != nil {
			return nil, err
		}
		ts, err = ts.ParseFS(ui.Files, page)
		if err != nil {
			return nil, err
		}
		cache[name] = ts
	}

	// Step 2: Cache partials
	partials, err := fs.Glob(ui.Files, "html/partials/*.html")
	if err != nil {
		return nil, err
	}
	for _, partial := range partials {
		name := filepath.Base(partial)
		ts, err := template.New(name).Funcs(functions).ParseFS(ui.Files, "html/partials/*.html")
		if err != nil {
			return nil, err
		}
		cache[name] = ts
	}

	return cache, nil
}
