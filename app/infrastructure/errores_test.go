package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net"
	"testing"

	"github.com/lib/pq"

	"voltrix/inventario/app/business/domain"
)

func TestTraducirError(t *testing.T) {
	otro := errors.New("columna inexistente")
	for _, caso := range []struct {
		nombre    string
		err       error
		categoria error
	}{
		{"sin filas", sql.ErrNoRows, domain.ErrNoEncontrado},
		{"clave duplicada", &pq.Error{Code: "23505"}, domain.ErrDuplicado},
		{"conexión perdida", &pq.Error{Code: "08006"}, domain.ErrNoDisponible},
		{"servidor apagándose", &pq.Error{Code: "57P01"}, domain.ErrNoDisponible},
		{"conexión rechazada", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, domain.ErrNoDisponible},
		{"tiempo agotado", context.DeadlineExceeded, domain.ErrNoDisponible},
	} {
		if err := traducirError(caso.err); !errors.Is(err, caso.categoria) || !errors.Is(err, caso.err) {
			t.Errorf("%s: %v no es %v o perdió la causa", caso.nombre, err, caso.categoria)
		}
	}

	if traducirError(nil) != nil {
		t.Error("nil debe seguir siendo nil")
	}
	sintaxis := &pq.Error{Code: "42601"}
	if err := traducirError(sintaxis); err != sintaxis {
		t.Errorf("un error de SQL no reconocido debe devolverse tal cual: %v", err)
	}
	if err := traducirError(otro); err != otro {
		t.Errorf("un error desconocido debe devolverse tal cual: %v", err)
	}
}

func TestMigracionesIncluidasEnElBinario(t *testing.T) {
	archivos, err := fs.Sub(archivosMigraciones, "migraciones")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Glob(archivos, "*.sql"); err != nil {
		t.Fatal(err)
	}
}
