package main

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strings"

	"warehouse-service/internal/config"
	"warehouse-service/internal/database"

	"golang.org/x/crypto/bcrypt"
)

type account struct {
	name     string
	email    string
	password string
	role     string
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	accounts, err := accountsFromEnvironment()
	if err != nil {
		return err
	}
	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, current := range accounts {
		hash, err := bcrypt.GenerateFromPassword([]byte(current.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash %s password: %w", current.role, err)
		}
		var userID int64
		if err := tx.QueryRow(`
			INSERT INTO users.users (name, email, password_hash)
			VALUES ($1, $2, $3)
			RETURNING id`, current.name, current.email, string(hash)).Scan(&userID); err != nil {
			return fmt.Errorf("create %s account (email may already exist): %w", current.role, err)
		}
		result, err := tx.Exec(`
			INSERT INTO users.user_has_roles (user_id, role_id)
			SELECT $1, id FROM users.roles WHERE name = $2`, userID, current.role)
		if err != nil {
			return fmt.Errorf("assign %s role: %w", current.role, err)
		}
		assigned, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("verify %s role: %w", current.role, err)
		}
		if assigned != 1 {
			return fmt.Errorf("required role %q is missing; apply database migrations first", current.role)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Println("Created GuKit admin and warehouse staff accounts.")
	return nil
}

func accountsFromEnvironment() ([]account, error) {
	adminEmail := normalizeEmail(os.Getenv("GUKIT_ADMIN_EMAIL"))
	staffEmail := normalizeEmail(os.Getenv("GUKIT_STAFF_EMAIL"))
	adminPassword := os.Getenv("GUKIT_ADMIN_PASSWORD")
	staffPassword := os.Getenv("GUKIT_STAFF_PASSWORD")
	if adminEmail == "" || staffEmail == "" || adminPassword == "" || staffPassword == "" {
		return nil, errors.New("set GUKIT_ADMIN_EMAIL, GUKIT_ADMIN_PASSWORD, GUKIT_STAFF_EMAIL, and GUKIT_STAFF_PASSWORD")
	}
	if adminEmail == staffEmail {
		return nil, errors.New("admin and warehouse staff emails must be different")
	}
	for _, current := range []struct {
		email    string
		password string
	}{
		{adminEmail, adminPassword},
		{staffEmail, staffPassword},
	} {
		address, err := mail.ParseAddress(current.email)
		if err != nil || address.Address != current.email {
			return nil, errors.New("account email is invalid")
		}
		if len(current.password) < 12 {
			return nil, errors.New("account passwords must contain at least 12 characters")
		}
	}
	return []account{
		{name: "GuKit Admin", email: adminEmail, password: adminPassword, role: "owner"},
		{name: "Staf Gudang", email: staffEmail, password: staffPassword, role: "staff"},
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
