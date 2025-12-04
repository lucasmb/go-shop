package handlers

import "net/http"

func (app *Application) ShowRegister(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, "register.page.html", data)
}

func (app *Application) DoRegister(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	name := r.PostForm.Get("name")
	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")

	// Basic validation
	if name == "" || email == "" || password == "" {
		app.SessionManager.Put(r.Context(), "flash", "All fields are required.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	id, err := app.Users.Insert(name, email, password)
	if err != nil {
		app.SessionManager.Put(r.Context(), "flash", "Email address is already in use.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	//Fetch the newly created user object from the database.
	user, err := app.Users.Get(id)
	if err != nil {
		// This is a server error, as the user we just created should exist.
		app.serverError(w, r, err)
		app.SessionManager.Put(r.Context(), "flash", "An unexpected error occurred. Please try logging in.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	//Renew the session token and store the *entire user object*, just like in DoLogin.
	app.SessionManager.RenewToken(r.Context())
	app.SessionManager.Put(r.Context(), "authenticatedUser", user)
	app.SessionManager.Put(r.Context(), "flash", "Registration successful! Welcome!")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Application) ShowLogin(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, "login.page.html", data)
}

func (app *Application) DoLogin(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")

	user, err := app.Users.Authenticate(email, password)
	if err != nil {
		app.SessionManager.Put(r.Context(), "flash", err.Error())
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.SessionManager.RenewToken(r.Context())
	app.SessionManager.Put(r.Context(), "authenticatedUser", user)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Application) DoLogout(w http.ResponseWriter, r *http.Request) {
	app.SessionManager.Destroy(r.Context())
	// An HTMX-powered logout should redirect the client.
	w.Header().Set("HX-Redirect", "/login")
}
