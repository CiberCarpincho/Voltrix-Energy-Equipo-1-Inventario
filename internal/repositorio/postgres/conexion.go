// Package postgres es la capa de acceso a datos: la única que conoce SQL y el driver de
// PostgreSQL. Implementa las interfaces (puertos) que declara internal/servicio y traduce los
// errores del driver a las categorías de internal/dominio.
//
// Cada historia de usuario agrega aquí su repositorio (por ejemplo activos.go) con consultas
// parametrizadas, y su migración en migraciones/.
package postgres

import (
	"database/sql"
	"time"

	// Registra el driver "postgres".
	_ "github.com/lib/pq"
)

// Abrir crea el pool de conexiones. Los datos de conexión salen de las variables PG*
// (PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD, PGSSLMODE, PGCONNECT_TIMEOUT).
// No abre ninguna conexión todavía: la primera ocurre al migrar.
func Abrir(maxConexiones int) (*sql.DB, error) {
	db, err := sql.Open("postgres", "")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxConexiones)
	db.SetMaxIdleConns(maxConexiones)
	db.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}
