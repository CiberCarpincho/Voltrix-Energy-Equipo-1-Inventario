package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rutaContrato() string {
	return filepath.Join("..", "..", "api", "openapi.yaml")
}

func TestContratoYDemoHU03(t *testing.T) {
	handler, err := New(nil, rutaContrato())
	if err != nil {
		t.Fatal(err)
	}
	contrato, err := os.ReadFile(rutaContrato())
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/openapi.yaml", nil))
	if w.Code != 200 || w.Body.String() != string(contrato) || w.Header().Get("Content-Type") != "application/yaml" {
		t.Fatalf("contrato publicado: HTTP %d", w.Code)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/activos/consulta",
		strings.NewReader(`{"ids":["PL-001-PS-07","PL-002-TB-01","PL-999-PS-01"]}`)))
	var lote struct {
		Encontrados  []struct{ ID string }
		Desconocidos []string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lote); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || w.Header().Get("X-Inventario-Demo") != "true" ||
		len(lote.Encontrados) != 2 || lote.Encontrados[0].ID != "PL-001-PS-07" ||
		len(lote.Desconocidos) != 1 || lote.Desconocidos[0] != "PL-999-PS-01" {
		t.Fatalf("consulta de ejemplo: HTTP %d %s", w.Code, w.Body)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/activos?limit=0", nil))
	if w.Code != http.StatusBadRequest || w.Header().Get("X-Request-Id") == "" {
		t.Fatalf("paginación inválida: HTTP %d %s", w.Code, w.Body)
	}
}
