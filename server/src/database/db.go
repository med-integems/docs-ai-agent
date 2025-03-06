package database

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/integems/report-agent/config"
	"github.com/integems/report-agent/src/models"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDatabaseConnection creates a new connection to the PostgreSQL database
func NewDatabaseConnection() *gorm.DB {
	// Get the PostgreSQL credentials from environment variables
	host := config.GetEnv("DB_HOST", "127.0.0.1")
	port := config.GetEnv("DB_PORT", "5432")
	user := config.GetEnv("DB_USER", "postgres")
	password := config.GetEnv("DB_PASSWORD", "postgres")
	dbname := config.GetEnv("DB_NAME", "agentdb")

	// Check if the database exists; create it if it does not
	ensureDatabaseExists(host, port, user, password, dbname)

	// Format the PostgreSQL connection string for the target database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

	// Open a new connection to the database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	return db
}

// ensureDatabaseExists checks if the database exists and creates it if it does not
func ensureDatabaseExists(host, port, user, password, dbname string) {
	// Connect to the default 'postgres' database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable", host, port, user, password)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to default database: %v", err)
	}
	defer db.Close()

	// Check if the target database exists
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = '%s')", dbname)
	err = db.QueryRow(query).Scan(&exists)
	if err != nil {
		log.Fatalf("Failed to check if database exists: %v", err)
	}

	// Create the database if it does not exist
	if !exists {
		_, err := db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbname))
		if err != nil {
			log.Fatalf("Failed to create database: %v", err)
		}
		log.Printf("Database %s created successfully", dbname)
	}
}

// AutoMigrateTables automatically migrates database tables
func AutoMigrateTables(db *gorm.DB) error {
	return db.AutoMigrate(&models.User{}, &models.Document{})
}
