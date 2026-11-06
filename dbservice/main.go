package main

import (
	"db/internal/database"
	httpapp "db/internal/http"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	connStr := os.Getenv("CONNECT")
	if connStr == "" {
		fmt.Println("no CONNECT from .env")
	}
	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("no PORT from .env")
	}
	pool, err := database.NewPool(connStr)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	router := httpapp.NewRouter(pool)
	http.ListenAndServe(port, router)

}
