package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"voltrix/inventario/internal/repositorio/postgres"
)

// New publica el contrato y sirve el catálogo del primer avance.
func New(db *sql.DB, contractPath string) (http.Handler, error) {
	contract, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	registrarCatalogo(mux, postgres.Catalogo{DB: db})
	if err := registrarEjemplos(mux, contract); err != nil {
		return nil, err
	}
	inicio := time.Now()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		salud := map[string]any{
			"estado": "disponible", "baseDatos": "ok", "version": "1.0.0",
			"tiempoActivoSeg": int64(time.Since(inicio) / time.Second),
		}
		if err := db.PingContext(ctx); err != nil {
			salud["estado"], salud["baseDatos"] = "no_disponible", "error"
			writeJSON(w, 503, salud)
			return
		}
		writeJSON(w, 200, salud)
	})
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(contract)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			http.Error(w, "Error al generar requestId", 500)
			return
		}
		w.Header().Set("X-Request-Id", hex.EncodeToString(id[:]))
		mux.ServeHTTP(w, r)
	}), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
