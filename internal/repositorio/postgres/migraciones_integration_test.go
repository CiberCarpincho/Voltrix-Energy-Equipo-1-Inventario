package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lib/pq"

	"voltrix/inventario/internal/dominio"
)

// abrirPrueba conecta a la base de INVENTARIO_TEST_DSN dentro de un esquema propio que se
// borra al terminar. Las pruebas que lo usan se omiten si la variable no está definida.
//
//	INVENTARIO_TEST_DSN="postgres://inventario:clave@127.0.0.1:5433/inventario?sslmode=disable" go test ./...
func abrirPrueba(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv("INVENTARIO_TEST_DSN")
	if dsn == "" {
		t.Skip("defina INVENTARIO_TEST_DSN para probar contra PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	// Una sola conexión: el search_path se fija por conexión.
	db.SetMaxOpenConns(1)

	esquema := pq.QuoteIdentifier(fmt.Sprintf("prueba_%d", time.Now().UnixNano()))
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+esquema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec("DROP SCHEMA " + esquema + " CASCADE"); err != nil {
			t.Errorf("borrar el esquema de prueba: %v", err)
		}
	})
	if _, err := db.ExecContext(ctx, "SET search_path TO "+esquema); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestMigrarAplicaCadaArchivoUnaVez(t *testing.T) {
	db, ctx := abrirPrueba(t)
	archivos := fstest.MapFS{
		"002_indice.sql": {Data: []byte("CREATE UNIQUE INDEX ejemplo_nombre ON ejemplo (nombre);")},
		"001_tabla.sql":  {Data: []byte("CREATE TABLE ejemplo (id serial PRIMARY KEY, nombre text NOT NULL);")},
		"LEAME.md":       {Data: []byte("no es una migración")},
	}

	aplicadas, err := aplicarMigraciones(ctx, db, archivos)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(aplicadas, []string{"001_tabla.sql", "002_indice.sql"}) {
		t.Fatalf("primera ejecución: %v", aplicadas)
	}
	if aplicadas, err = aplicarMigraciones(ctx, db, archivos); err != nil || len(aplicadas) != 0 {
		t.Fatalf("segunda ejecución: %v, %v", aplicadas, err)
	}

	// El índice único de la migración 002 se traduce a la categoría Duplicado.
	if _, err := db.ExecContext(ctx, "INSERT INTO ejemplo (nombre) VALUES ('a')"); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO ejemplo (nombre) VALUES ('a')")
	if !errors.Is(traducirError(err), dominio.ErrDuplicado) {
		t.Fatalf("clave duplicada: %v", err)
	}
	if err := (Salud{DB: db}).Verificar(ctx); err != nil {
		t.Fatalf("salud con la base disponible: %v", err)
	}
}

func TestMigrarRevierteSiUnaFalla(t *testing.T) {
	db, ctx := abrirPrueba(t)
	_, err := aplicarMigraciones(ctx, db, fstest.MapFS{
		"001_tabla.sql":  {Data: []byte("CREATE TABLE ejemplo (id int);")},
		"002_errada.sql": {Data: []byte("CREATE TABLA mal_escrita ();")},
	})
	if err == nil {
		t.Fatal("se esperaba un error de sintaxis")
	}
	var existe bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('ejemplo') IS NOT NULL").Scan(&existe); err != nil {
		t.Fatal(err)
	}
	if existe {
		t.Fatal("la migración 001 no debía quedar aplicada")
	}
}
