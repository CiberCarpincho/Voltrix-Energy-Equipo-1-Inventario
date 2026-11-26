package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// New sirve el ejemplo del contrato para el Avance 1 y comprueba PostgreSQL en cada consulta de salud.
func New(db *sql.DB, contractPath string) (http.Handler, error) {
	contract, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, err
	}
	var spec struct {
		Components struct {
			Examples map[string]struct {
				Value json.RawMessage
			}
		}
	}
	if err := json.Unmarshal(contract, &spec); err != nil {
		return nil, err
	}
	panel := spec.Components.Examples["ActivoPanel"].Value
	var asset struct{ ID string }
	if err := json.Unmarshal(panel, &asset); err != nil || asset.ID == "" {
		return nil, fmt.Errorf("el contrato debe contener el ejemplo ActivoPanel con id")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			writeJSON(w, 503, map[string]string{"estado": "no_disponible", "baseDatos": "error"})
			return
		}
		writeJSON(w, 200, map[string]string{"estado": "disponible", "baseDatos": "ok"})
	})
	mux.HandleFunc("GET /activos/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != asset.ID {
			writeJSON(w, 404, map[string]string{"codigo": "NO_ENCONTRADO", "mensaje": "Activo desconocido", "requestId": w.Header().Get("X-Request-Id")})
			return
		}
		writeJSON(w, 200, panel)
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
