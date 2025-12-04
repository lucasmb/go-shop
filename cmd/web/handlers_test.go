package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeHandler(t *testing.T) {
	// Create a new instance of our test application.
	// In a real scenario, you'd inject a test DB here.
	app := newTestApplication(t)

	// Create an httptest.Server to serve our application.
	ts := httptest.NewServer(app.routes())
	defer ts.Close()

	// Use the test server's client to make a request to the Home endpoint.
	rs, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is 200 OK.
	if rs.StatusCode != http.StatusOK {
		t.Errorf("expected status %d; got %d", http.StatusOK, rs.StatusCode)
	}
}
