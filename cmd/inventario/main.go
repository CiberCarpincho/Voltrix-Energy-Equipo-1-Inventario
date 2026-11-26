package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	catalogodb "voltrix/inventario/db"
	"voltrix/inventario/internal/httpapi"
)

func main() {
	db, err := sql.Open("postgres", "")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err = catalogodb.Inicializar(ctx, db)
	cancel()
	if err != nil {
		log.Fatalf("inicializar catálogo: %v", err)
	}

	handler, err := httpapi.New(db, "api/openapi.yaml")
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
