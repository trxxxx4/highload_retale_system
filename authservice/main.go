package main

import (
	"auth/internal/database"
	dbclient "auth/internal/dbservice"
	httpApp "auth/internal/http"
	"auth/internal/service"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	connStr := os.Getenv("CONNECT")
	if connStr == "" {
		fmt.Println("no CONNECT from .env")
	}
	pool, err := database.NewPool(connStr)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	migrateVersionString := os.Getenv("MIGRATE_VERSION")
	if connStr == "" {
		fmt.Println("no MIGRATE_VERSION from .env")
	}
	migrateVersion, err := strconv.Atoi(migrateVersionString)
	if err != nil {
		panic("not able to convent MIGRATE_VERSION to int")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	issuer := os.Getenv("JWT_ISSUER")
	if secret == "" {
		log.Fatal("JWT_ISSUER is required")
	}

	ttl, err := time.ParseDuration(
		os.Getenv("JWT_TTL"),
	)
	if err != nil {
		log.Fatal("invalid JWT_TTL")
	}

	jwtManager, err := service.NewJWTManager(
		secret,
		ttl,
		issuer,
	)
	if err != nil {
		log.Fatal(err)
	}

	url := os.Getenv("DBSERVICE_URL")
	if url == "" {
		fmt.Println("no DBSERVICE_URL from .env")
	}

	clientHTTP := dbclient.NewHTTPClient()
	clientDB := dbclient.NewClient(url, clientHTTP)

	router := httpApp.NewRouter(pool, migrateVersion, jwtManager, clientDB)
	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("no PORT from .env")
	}
	http.ListenAndServe(port, router)

}
