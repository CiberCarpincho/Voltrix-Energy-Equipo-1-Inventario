package infraestructure

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net"

	"github.com/lib/pq"

	"voltrix/inventario/app/business/domain"
)

// Código de PostgreSQL para una violación de unicidad (clave primaria o índice único).
const violacionUnicidad = "23505"

// traducirError convierte un error del driver en una categoría de dominio, para que las capas
// de arriba no conozcan sql ni pq. Los errores que no reconoce se devuelven tal cual (500).
func traducirError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &domain.Error{Categoria: domain.ErrNoEncontrado, Causa: err}
	}

	var errPostgres *pq.Error
	if errors.As(err, &errPostgres) {
		switch {
		case errPostgres.Code == violacionUnicidad:
			return &domain.Error{Categoria: domain.ErrDuplicado, Causa: err}
		// 08: excepción de conexión; 57: el servidor se está apagando o reiniciando.
		case errPostgres.Code.Class() == "08", errPostgres.Code.Class() == "57":
			return domain.NoDisponible(err)
		}
		return err
	}

	var errRed net.Error
	if errors.As(err, &errRed) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) {
		return domain.NoDisponible(err)
	}
	return err
}
