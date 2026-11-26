// Package servicio es la capa de lógica de negocio: casos de uso del catálogo, validaciones de
// tipos, estados y orden temporal, y coordinación de los lotes con el pool de internal/lotes.
//
// No conoce HTTP ni SQL. Lo que necesita de la capa de acceso a datos lo declara como
// interfaces (puertos) en este archivo; internal/repositorio/postgres las implementa y
// cmd/inventario conecta una con otra. En las pruebas se reemplazan por dobles.
package servicio

import "context"

// VerificadorBaseDatos comprueba que la base de datos responde.
type VerificadorBaseDatos interface {
	Verificar(ctx context.Context) error
}

// Cada historia de usuario agrega aquí el puerto de su repositorio, por ejemplo:
//
//	type RepositorioActivos interface {
//		Buscar(ctx context.Context, id string) (dominio.Activo, error)
//	}
