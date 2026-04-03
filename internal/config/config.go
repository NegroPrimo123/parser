package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DB     DBConfig
	Parser ParserConfig
	Server ServerConfig
	Logger LoggerConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type ParserConfig struct {
	SearchQuery      string
	Pages            int
	VacanciesPerPage int
	Workers          int
	RetryAttempts    int
	CSVEnabled       bool
}

type ServerConfig struct {
	Addr string
}

type LoggerConfig struct {
	Level      string
	JSONFormat bool
}

func Load() (*Config, error) {
	cfg := &Config{
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "parser"),
			Password: getEnv("DB_PASSWORD", "password"),
			Name:     getEnv("DB_NAME", "hh_parser"),
		},
		Parser: ParserConfig{
			SearchQuery:      getEnv("SEARCH_QUERY", "Golang developer"),
			Pages:            getEnvInt("PAGES", 2),
			VacanciesPerPage: getEnvInt("VACANCIES_PER_PAGE", 10),
			Workers:          getEnvInt("WORKERS", 3),
			RetryAttempts:    getEnvInt("RETRY_ATTEMPTS", 5),
			CSVEnabled:       getEnv("CSV_ENABLED", "true") == "true",
		},
		Server: ServerConfig{
			Addr: getEnv("HTTP_ADDR", ":8080"),
		},
		Logger: LoggerConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			JSONFormat: getEnv("LOG_FORMAT", "text") == "json",
		},
	}
	
	// Валидация
	if cfg.Parser.Pages <= 0 {
		return nil, fmt.Errorf("PAGES must be positive, got %d", cfg.Parser.Pages)
	}
	if cfg.Parser.VacanciesPerPage <= 0 || cfg.Parser.VacanciesPerPage > 100 {
		return nil, fmt.Errorf("VACANCIES_PER_PAGE must be between 1 and 100, got %d", cfg.Parser.VacanciesPerPage)
	}
	if cfg.Parser.Workers <= 0 {
		return nil, fmt.Errorf("WORKERS must be positive, got %d", cfg.Parser.Workers)
	}
	if cfg.Parser.RetryAttempts <= 0 {
		return nil, fmt.Errorf("RETRY_ATTEMPTS must be positive, got %d", cfg.Parser.RetryAttempts)
	}
	
	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}