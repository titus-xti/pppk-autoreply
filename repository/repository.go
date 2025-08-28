package repository

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// dsn holds the connection string to the Postgres database.
// TODO: move to config/env
const dsn = "postgres://postgres:Gkjp2025@134.209.100.169:5432/vote?sslmode=disable"

var pool *pgxpool.Pool

// Init initializes the pgx connection pool
func Init(ctx context.Context) error {
	if pool != nil {
		return nil
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("new pool: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return fmt.Errorf("ping pool: %w", err)
	}
	pool = p
	return nil
}

// Close closes the pool; safe to call multiple times
func Close() {
	if pool != nil {
		pool.Close()
		pool = nil
	}
}

// VoteMasterPhoneExists returns the name for the given phone from vote_master.
func VoteMasterPhoneExists(phone string) (string, error) {
	if pool == nil {
		return "", errors.New("repository not initialized: call repository.Init")
	}
	var name string
	err := pool.QueryRow(context.Background(), "SELECT name FROM vote_master WHERE phone = $1 LIMIT 1", phone).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("vote_master check error: %v", err)
		return "", nil
	}
	if err != nil {
		log.Printf("vote_master check error: %v", err)
		return "", err
	}
	return name, nil
}

// GetVoteMaster param phone
func GetVoteMaster(phone string) (name, wilayah, dob string, found bool, err error) {
	if pool == nil {
		return "", "", "", false, errors.New("repository not initialized: call repository.Init")
	}
	err = pool.QueryRow(context.Background(), "SELECT name, wilayah, extract(year from dob)::text dob FROM vote_master WHERE phone = $1", phone).
		Scan(&name, &wilayah, &dob)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("vote_master check error: %v", err)
		return "", "", "", false, nil
	}
	if err != nil {
		log.Printf("vote_master check error: %v", err)
		return "", "", "", false, err
	}
	return name, wilayah, dob, true, nil
}

// GetRegistrationByPhone returns registration data for phone_number if exists
func GetRegistrationByPhone(phone string) (name, wilayah, year string, found bool, err error) {
	if pool == nil {
		return "", "", "", false, errors.New("repository not initialized: call repository.Init")
	}
	err = pool.QueryRow(context.Background(), "SELECT name, wilayah, year_of_birth FROM registration WHERE phone_number = $1", phone).
		Scan(&name, &wilayah, &year)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("registration check error: %v", err)
		return "", "", "", false, nil
	}
	if err != nil {
		log.Printf("registration check error: %v", err)
		return "", "", "", false, err
	}
	return name, wilayah, year, true, nil
}

// InsertRegistration inserts a new registration row
func InsertRegistration(name, wilayah, year, phone string) error {
	if pool == nil {
		return errors.New("repository not initialized: call repository.Init")
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO registration (name, wilayah, year_of_birth, phone_number, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, name, wilayah, year, phone)
	if err != nil {
		log.Printf("insert registration error: %v", err)
		return fmt.Errorf("insert registration: %w", err)
	}
	return nil
}

// InsertVoters inserts a new voters row
func InsertVoters(name, phone string) (string, error) {

	code := uuid.New().String()

	if pool == nil {
		return "", errors.New("repository not initialized: call repository.Init")
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO voters (code, name, phone)
		VALUES ($1, $2, $3)
	`, code, name, phone)
	if err != nil {
		log.Printf("insert voters error: %v", err)
		return "", fmt.Errorf("insert voters: %w", err)
	}
	return code, nil
}

// GetVoterCodeByPhone returns the code for the given phone from voters.
func GetVoterCodeByPhone(phone string) (string, error) {
	if pool == nil {
		return "", errors.New("repository not initialized: call repository.Init")
	}
	var code string
	err := pool.QueryRow(context.Background(), "SELECT code FROM voters WHERE phone = $1 LIMIT 1", phone).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("voter check error: %v", err)
		return "", nil
	}
	if err != nil {
		log.Printf("voter check error: %v", err)
		return "", err
	}
	return code, nil
}
