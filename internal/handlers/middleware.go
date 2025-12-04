package handlers

import (
	"context"
	"go-shop/internal/data"
	"net/http"
)

type contextKey string

const contextKeyUser = contextKey("user")

// PopulateUser gets the user from the session and adds them to the request context.
func (app *Application) PopulateUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the full user object directly from the session.
		user, ok := app.SessionManager.Get(r.Context(), "authenticatedUser").(*data.User)
		if !ok {
			// User is not logged in.
			next.ServeHTTP(w, r)
			return
		}

		// Add user to the request context.
		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuthentication redirects users to the login page if they are not logged in.
func (app *Application) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(contextKeyUser).(*data.User)
		if !ok || user == nil {
			app.SessionManager.Put(r.Context(), "flash", "You must be logged in to access this page.")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin ensures the user is an administrator.
func (app *Application) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(contextKeyUser).(*data.User)
		if !ok || user == nil || !user.IsAdmin {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Middleware chain helper
type Chain []func(http.Handler) http.Handler

func NewChain(middlewares ...func(http.Handler) http.Handler) Chain {
	return middlewares
}

func (c Chain) Then(h http.Handler) http.Handler {
	for i := range c {
		h = c[len(c)-1-i](h)
	}
	return h
}

func (c Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	return c.Then(fn)
}

func (c Chain) Append(middlewares ...func(http.Handler) http.Handler) Chain {
	newChain := make(Chain, len(c)+len(middlewares))
	copy(newChain, c)
	copy(newChain[len(c):], middlewares)
	return newChain
}
