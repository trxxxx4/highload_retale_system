package main

import (
	"context"
	httpApp "inventory/internal/http"
	"inventory/internal/http/dbclient"
	"inventory/internal/http/middleware"
	"inventory/internal/redis"
	"inventory/internal/worker"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		log.Fatal("JWT_ISSUER is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("PORT is required")
	}
	url := os.Getenv("DBSERVICE_URL")
	if url == "" {
		log.Fatal("DBSERVICE_URL is required")
	}

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		log.Print("REDIS_ADDR is required")
		addr = "localhost:6379"
	}

	password := os.Getenv("REDIS_PASSWORD")

	clientRedis := redis.NewClient(addr, password, 0)
	defer clientRedis.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)

	err := clientRedis.Ping(ctx).Err()
	cancel()

	if err != nil {
		log.Printf(
			"Redis unavailable, starting without cache: %v",
			err,
		)
	} else {
		log.Println("Redis connected successfully")
	}

	clientHTTP := dbclient.NewHTTPClient()
	clientDB := dbclient.NewClient(url, clientHTTP)

	auth := middleware.NewAuthMiddleware(secret, issuer)
	router := httpApp.NewRouter(auth, clientDB, clientRedis)

	workerCtx, cancelWorker := context.WithCancel(
		context.Background(),
	)
	defer cancelWorker()

	expiryWorker := worker.NewExpiryWorker(clientDB, clientRedis, 10*time.Second, 100)
	go expiryWorker.Run(workerCtx)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}

}
