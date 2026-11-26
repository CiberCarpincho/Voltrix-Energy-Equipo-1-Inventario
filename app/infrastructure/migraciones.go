package infrastructure

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

// Los archivos de migraciones/ se incluyen en el binario. Se nombran NNN_descripcion.sql
// (001_activos.sql, 002_...) y se aplican en orden alfabético, una sola vez cada uno.
// Una migración ya aplicada no se edita: los cambios van en una migración nueva.
//
//go:embed all:migraciones
var archivosMigraciones embed.FS

// Clave del bloqueo que serializa las migraciones si arrancan varias réplicas a la vez.
const bloqueoMigraciones = 1270080

// Migrar aplica las migraciones pendientes y devuelve los nombres de las que aplicó.
func Migrar(ctx context.Context, db *sql.DB) ([]string, error) {
	archivos, err := fs.Sub(archivosMigraciones, "migraciones")
	if err != nil {
		return nil, err
	}
	return aplicarMigraciones(ctx, db, archivos)
}

// aplicarMigraciones aplica, en una sola transacción, los .sql de archivos que aún no
// figuran en la tabla migraciones_aplicadas.
func aplicarMigraciones(ctx context.Context, db *sql.DB, archivos fs.FS) ([]string, error) {
	nombres, err := fs.Glob(archivos, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(nombres)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, traducirError(err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", bloqueoMigraciones); err != nil {
		return nil, traducirError(err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS migraciones_aplicadas (
		version     text PRIMARY KEY,
		aplicada_en timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return nil, traducirError(err)
	}

	var aplicadas []string
	for _, nombre := range nombres {
		var yaAplicada bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM migraciones_aplicadas WHERE version = $1)", nombre,
		).Scan(&yaAplicada); err != nil {
			return nil, traducirError(err)
		}
		if yaAplicada {
			continue
		}
		contenido, err := fs.ReadFile(archivos, nombre)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, string(contenido)); err != nil {
			return nil, fmt.Errorf("aplicar la migración %s: %w", nombre, traducirError(err))
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO migraciones_aplicadas (version) VALUES ($1)", nombre); err != nil {
			return nil, traducirError(err)
		}
		aplicadas = append(aplicadas, nombre)
	}
	if err := tx.Commit(); err != nil {
		return nil, traducirError(err)
	}
	return aplicadas, nil
}
