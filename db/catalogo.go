package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed migraciones/001_catalogo.sql
var esquema string

//go:embed semillas/001_catalogo.sql
var semilla string

// Inicializar aplica el esquema y carga el catálogo inicial en una sola transacción.
func Inicializar(ctx context.Context, conexion *sql.DB) error {
	tx, err := conexion.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Serializa el arranque si varias instancias comparten la base.
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1270080)"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, esquema); err != nil {
		return fmt.Errorf("crear esquema: %w", err)
	}
	var vacia bool
	if err := tx.QueryRowContext(ctx, "SELECT NOT EXISTS (SELECT 1 FROM plantas)").Scan(&vacia); err != nil {
		return err
	}
	if vacia {
		if _, err := tx.ExecContext(ctx, semilla); err != nil {
			return fmt.Errorf("cargar semilla: %w", err)
		}
	}
	return tx.Commit()
}
