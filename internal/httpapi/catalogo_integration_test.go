package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
	catalogodb "voltrix/inventario/db"
)

func TestCatalogoPostgresHU03(t *testing.T) {
	dsn := os.Getenv("INVENTARIO_TEST_DSN")
	if dsn == "" {
		t.Skip("defina INVENTARIO_TEST_DSN para probar con PostgreSQL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	esquema := pq.QuoteIdentifier(fmt.Sprintf("hu03_%d", time.Now().UnixNano()))
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+esquema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP SCHEMA " + esquema + " CASCADE"); err != nil {
			t.Errorf("limpiar esquema de prueba: %v", err)
		}
	}()

	abrir := func() *sql.DB {
		t.Helper()
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		db.SetMaxOpenConns(1)
		if _, err := db.ExecContext(ctx, "SET search_path TO "+esquema); err != nil {
			t.Fatal(err)
		}
		if err := catalogodb.Inicializar(ctx, db); err != nil {
			t.Fatal(err)
		}
		return db
	}

	consultar := func(db *sql.DB, ruta string, status int, destino any) {
		t.Helper()
		handler, err := New(db, rutaContrato())
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", ruta, nil))
		if w.Code != status || w.Header().Get("X-Request-Id") == "" {
			t.Fatalf("GET %s: HTTP %d %s", ruta, w.Code, w.Body)
		}
		if err := json.Unmarshal(w.Body.Bytes(), destino); err != nil {
			t.Fatal(err)
		}
	}

	db := abrir()
	defer func() { db.Close() }()
	var activo struct{ ID, PlantaID, Estado string }
	consultar(db, "/activos/PL-001-PS-07", http.StatusOK, &activo)
	if activo.ID != "PL-001-PS-07" || activo.PlantaID != "PL-001" || activo.Estado != "operativo" {
		t.Fatalf("activo inicial: %+v", activo)
	}

	var pagina struct {
		Total int
		Datos []struct{ ID string }
	}
	for _, caso := range []struct {
		ruta  string
		total int
		id    string
	}{
		{"/plantas?limit=1", 80, "PL-001"},
		{"/plantas?limit=1&tipo=solar", 40, "PL-001"},
		{"/plantas?limit=1&tipo=eolica", 25, "PL-002"},
		{"/plantas?limit=1&tipo=hibrida", 15, "PL-003"},
		{"/activos?limit=1", 1270, "PL-001-BB-01"},
		{"/activos?limit=1&offset=1&plantaId=PL-001&tipo=panel_solar&estado=operativo", 20, "PL-001-PS-02"},
	} {
		consultar(db, caso.ruta, http.StatusOK, &pagina)
		if pagina.Total != caso.total || len(pagina.Datos) != 1 || pagina.Datos[0].ID != caso.id {
			t.Fatalf("GET %s: %+v", caso.ruta, pagina)
		}
	}
	var planta struct {
		ID      string
		Resumen struct{ TotalActivos int }
	}
	consultar(db, "/plantas/PL-003", http.StatusOK, &planta)
	if planta.ID != "PL-003" || planta.Resumen.TotalActivos != 16 {
		t.Fatalf("planta de ejemplo: %+v", planta)
	}
	var errorAPI struct{ Codigo string }
	consultar(db, "/activos/PL-999-PS-01", http.StatusNotFound, &errorAPI)
	if errorAPI.Codigo != "NO_ENCONTRADO" {
		t.Fatalf("error inesperado: %+v", errorAPI)
	}

	if _, err := db.ExecContext(ctx, "UPDATE activos SET estado = 'en_mantenimiento' WHERE id = 'PL-001-PS-07'"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = abrir()
	consultar(db, "/activos/PL-001-PS-07", http.StatusOK, &activo)
	consultar(db, "/activos?limit=1", http.StatusOK, &pagina)
	if activo.Estado != "en_mantenimiento" || pagina.Total != 1270 {
		t.Fatalf("catálogo después del reinicio: %+v, total=%d", activo, pagina.Total)
	}
}
