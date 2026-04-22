package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/denisenkom/go-mssqldb"
)

var DB *sql.DB

func Connect() {
	user := getEnv("DB_USER", "db47937")
	password := getEnv("DB_PASSWORD", "5t#LE-8c+xD3")
	server := getEnv("DB_SERVER", "db47937.public.databaseasp.net")
	database := getEnv("DB_NAME", "db47937")

	connString := fmt.Sprintf(
		"server=%s;user id=%s;password=%s;database=%s;encrypt=disable;trustServerCertificate=true",
		server, user, password, database,
	)

	var err error
	DB, err = sql.Open("sqlserver", connString)
	if err != nil {
		log.Fatalf("Error abriendo conexión SQL Server: %v", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("Error conectando a SQL Server: %v", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)

	log.Println("SQL Server conectado")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
