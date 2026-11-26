package infrastructure

import (
	"context"
	"database/sql"
)

// Salud comprueba que PostgreSQL responde. Implementa services.VerificadorBaseDatos.
type Salud struct {
	DB *sql.DB
}

// Verificar hace un ping a la base con el tiempo de espera del contexto.
func (s Salud) Verificar(ctx context.Context) error {
	return traducirError(s.DB.PingContext(ctx))
}
