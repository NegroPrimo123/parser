package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hh-parser/internal/api"
	"hh-parser/internal/config"
	"hh-parser/internal/exporter"
	"hh-parser/internal/health"
	"hh-parser/internal/parser"
	"hh-parser/internal/server"
	"hh-parser/internal/service"
	"hh-parser/internal/storage"
	"hh-parser/pkg/logger"
	"hh-parser/pkg/retry"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func createHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}
}

func main() {
	// Загрузка .env файла
	if err := godotenv.Load(); err != nil {
		logger.Log.Warn("No .env file found, using environment variables")
	}

	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", err)
	}

	// Инициализация логгера
	logger.Init(cfg.Logger.Level, cfg.Logger.JSONFormat)
	logger.Log.Info("Starting HH.ru Parser", "version", "2.0.0")

	// Проверка connectivity
	if err := checkConnectivity(); err != nil {
		logger.Log.Warn("Connectivity check failed", "error", err)
	}

	// Подключение к БД
	logger.Log.Info("Connecting to database", "host", cfg.DB.Host, "port", cfg.DB.Port, "dbname", cfg.DB.Name)

	storageInstance, err := storage.NewPostgresStorage(cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name)
	if err != nil {
		logger.Fatal("Failed to connect to database", err)
	}
	defer storageInstance.Close()
	logger.Log.Info("Database connected successfully")

	// Инициализация health checks
	health.Init(storageInstance.DB())
	health.SetReady(true)

	// CSV экспортер
	var csvExporter *exporter.CSVExporter
	if cfg.Parser.CSVEnabled {
		logger.Log.Info("Creating CSV exporter")
		csvExporter, err = exporter.NewCSVExporter("")
		if err != nil {
			logger.Fatal("Failed to create CSV exporter", err)
		}
		defer csvExporter.Close()
		logger.Log.Info("CSV exporter created", "path", csvExporter.GetFilePath())
	} else {
		logger.Log.Info("CSV export disabled")
	}

	// Парсер HH
	hhParser := parser.NewHHParserWithClient(createHTTPClient())

	// Настройка retry
	retryCfg := retry.DefaultConfig()
	retryCfg.MaxAttempts = cfg.Parser.RetryAttempts
	hhParser.SetRetryConfig(retryCfg)

	// Сервис парсера
	parserService := service.NewParserService(
		hhParser,
		storageInstance,
		csvExporter,
		cfg.Parser.Workers,
	)

	logger.Log.Info("Search configuration",
		"query", cfg.Parser.SearchQuery,
		"pages", cfg.Parser.Pages,
		"vacancies_per_page", cfg.Parser.VacanciesPerPage,
		"workers", cfg.Parser.Workers,
		"retry_attempts", cfg.Parser.RetryAttempts,
	)

	// Настройка HTTP сервера
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.Handler)
	mux.HandleFunc("GET /ready", health.ReadyHandler)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /export/csv", api.ExportCSVHandler(storageInstance))

	httpServer := server.NewServer(cfg.Server.Addr, mux)
	httpServer.Start()

	// Контекст для graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Запуск парсинга
	logger.Log.Info("Starting parsing...")
	result, err := parserService.Run(ctx, cfg.Parser.SearchQuery, cfg.Parser.Pages, cfg.Parser.VacanciesPerPage)
	if err != nil {
		logger.Log.Error("Parser failed", "error", err)
		os.Exit(1)
	}

	// Вывод результатов
	printResults(result)

	// Graceful shutdown HTTP сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("HTTP server shutdown error", "error", err)
	}

	logger.Log.Info("Parser finished successfully")
}

func checkConnectivity() error {
	conn, err := net.DialTimeout("tcp", "api.hh.ru:443", 10*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to api.hh.ru: %w", err)
	}
	conn.Close()
	return nil
}

func printResults(result *service.ParseResult) {
	fmt.Println("\n╔══════════════════════════════════════════════════╗")
	fmt.Println("║                 РЕЗУЛЬТАТЫ                       ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Printf("Найдено вакансий: %d\n", result.TotalFound)
	fmt.Printf("Успешно обработано: %d\n", result.ParsedCount)
	fmt.Printf("Ошибок: %d\n", len(result.Errors))

	if len(result.Errors) > 0 {
		fmt.Println("Последние ошибки:")
		maxErrors := 5
		if len(result.Errors) < maxErrors {
			maxErrors = len(result.Errors)
		}
		for i := 0; i < maxErrors; i++ {
			fmt.Printf("   - %v\n", result.Errors[i])
		}
		if len(result.Errors) > 5 {
			fmt.Printf("   ... и ещё %d ошибок\n", len(result.Errors)-5)
		}
	}
}
