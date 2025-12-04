package data

import (
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserModel struct {
	DB     *sql.DB
	Logger *slog.Logger
}

func (m *UserModel) Insert(name, email, password string) (int64, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, err
	}

	stmt := `INSERT INTO users (name, email, hashed_password, created_at) VALUES (?, ?, ?, ?)`
	result, err := m.DB.Exec(stmt, name, email, string(hashedPassword), time.Now())
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (m *UserModel) Authenticate(email, password string) (*User, error) {
	var user User
	stmt := `SELECT id, name, email, hashed_password, is_admin, created_at FROM users WHERE email = ?`
	err := m.DB.QueryRow(stmt, email).Scan(&user.ID, &user.Name, &user.Email, &user.HashedPassword, &user.IsAdmin, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	return &user, nil
}

func (m *UserModel) Get(id int64) (*User, error) {
	var user User
	stmt := `SELECT id, name, email, is_admin, created_at FROM users WHERE id = ?`
	err := m.DB.QueryRow(stmt, id).Scan(&user.ID, &user.Name, &user.Email, &user.IsAdmin, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
