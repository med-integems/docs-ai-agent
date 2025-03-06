package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type EnvValue interface {
	string | int
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file", err.Error())
	}
}

func GetEnv(name string, fallback string) string {
	env := os.Getenv(name)
	if env != "" {
		return env
	}
	return fallback
}
