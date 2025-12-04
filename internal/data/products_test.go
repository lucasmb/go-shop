package data

import (
	"go-shop/internal/testutils"
	"testing"
)

func TestProductModel_Search(t *testing.T) {
	db := testutils.NewTestDB(t)
	logger := testutils.NewTestLogger()
	m := ProductModel{DB: db, Logger: logger}

	tests := []struct {
		name              string
		filters           SearchFilters
		expectedTotal     int
		expectedPageCount int
		expectedFirstName string
	}{
		{
			name:              "no filters, default sort (newest first)",
			filters:           SearchFilters{Page: 1, PageSize: 10},
			expectedTotal:     5,
			expectedPageCount: 5,
			expectedFirstName: "Go Gopher Pin", // Newest item from 2023-04-01
		},
		{
			name:              "filter by category 'Apparel'",
			filters:           SearchFilters{Category: "Apparel", Page: 1, PageSize: 10},
			expectedTotal:     2,
			expectedPageCount: 2,
			expectedFirstName: "HTMX T-Shirt", // Newest in Apparel
		},
		{
			name:              "filter by search query 'Book'",
			filters:           SearchFilters{Query: "Book", Page: 1, PageSize: 10},
			expectedTotal:     2,
			expectedPageCount: 2,
			expectedFirstName: "Go Book", // Newest book
		},
		{
			name:              "sort by price ascending",
			filters:           SearchFilters{Sort: "price_asc", Page: 1, PageSize: 10},
			expectedTotal:     5,
			expectedPageCount: 5,
			expectedFirstName: "Go Gopher Pin", // Cheapest item at 799
		},
		{
			name:              "pagination (page 2 of 2 items per page, sorted by price)",
			filters:           SearchFilters{Sort: "price_asc", Page: 2, PageSize: 2},
			expectedTotal:     5,
			expectedPageCount: 2,
			expectedFirstName: "Go T-Shirt", // 3rd cheapest item is the Go T-Shirt at 2999
		},
		{
			name:              "combined filter (Apparel, under $25)",
			filters:           SearchFilters{Category: "Apparel", MaxPrice: 25, Page: 1, PageSize: 10},
			expectedTotal:     1,
			expectedPageCount: 1,
			expectedFirstName: "HTMX T-Shirt", // Only the HTMX shirt is under $25 in apparel
		},
		{
			name:              "filter by search query for variant 'Large'",
			filters:           SearchFilters{Query: "Large", Page: 1, PageSize: 10},
			expectedTotal:     1,
			expectedPageCount: 1,
			expectedFirstName: "HTMX T-Shirt",
		},
		{
			name:              "no results found",
			filters:           SearchFilters{Query: "nonexistent", Page: 1, PageSize: 10},
			expectedTotal:     0,
			expectedPageCount: 0,
			expectedFirstName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			products, totalRecords, err := m.Search(tt.filters)
			if err != nil {
				t.Fatalf("Search() returned an error: %v", err)
			}

			if totalRecords != tt.expectedTotal {
				t.Errorf("expected %d total records; got %d", tt.expectedTotal, totalRecords)
			}

			if len(products) != tt.expectedPageCount {
				t.Errorf("expected %d products on this page; got %d", tt.expectedPageCount, len(products))
			}

			if tt.expectedFirstName != "" && (len(products) == 0 || products[0].Name != tt.expectedFirstName) {
				var gotName string
				if len(products) > 0 {
					gotName = products[0].Name
				}
				t.Errorf("expected first product name to be '%s'; got '%s'", tt.expectedFirstName, gotName)
			}
		})
	}
}
