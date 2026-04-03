package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var dsn string
	var cmd string

	if envDSN := os.Getenv("DATABASE_URL"); envDSN != "" {
		dsn = envDSN
	}

	flag.StringVar(&dsn, "dsn", dsn, "Postgres DSN")
	flag.StringVar(&cmd, "cmd", "up", "migration command: up, down, force, version")
	flag.Parse()

	if dsn == "" {
		log.Fatal("DSN is required (set DATABASE_URL env or -dsn flag)")
	}

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		log.Fatal("Failed to create migrate instance:", err)
	}
	defer m.Close()

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatal("Migration up failed:", err)
		}
		log.Println("Migration up completed successfully")
		
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatal("Migration down failed:", err)
		}
		log.Println("Migration down completed successfully")
		
	case "force":
		if flag.NArg() < 1 {
			log.Fatal("version required for force command")
		}
		version := flag.Arg(0)
		var v int
		if _, err := fmt.Sscanf(version, "%d", &v); err != nil {
			log.Fatal("Invalid version:", err)
		}
		if err := m.Force(v); err != nil {
			log.Fatal("Migration force failed:", err)
		}
		log.Printf("Migration force to version %d completed", v)
		
	case "version":
		version, dirty, err := m.Version()
		if err != nil && err != migrate.ErrNilVersion {
			log.Fatal("Failed to get version:", err)
		}
		if err == migrate.ErrNilVersion {
			log.Println("No migrations applied")
		} else {
			log.Printf("Current version: %d, dirty: %v", version, dirty)
		}
		
	default:
		log.Fatalf("Unknown command: %s", cmd)
	}
}