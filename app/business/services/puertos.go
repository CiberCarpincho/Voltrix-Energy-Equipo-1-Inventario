// Package services es la capa de lógica de negocio: casos de uso del catálogo, validaciones de
// tipos, estados y orden temporal, y coordinación de los lotes con el pool de app/business/workers.
//
// No conoce HTTP ni SQL. Lo que necesita de la capa de acceso a datos lo declara como
// interfaces (puertos) en este archivo; app/infraestructure las implementa y
// app/main.go conecta una con otra. En las pruebas se reemplazan por dobles.
package services

import "context"

// VerificadorBaseDatos comprueba que la base de datos responde.
type VerificadorBaseDatos interface {
	Verificar(ctx context.Context) error
}

// Cada historia de usuario agrega aquí el puerto de su repositorio, por ejemplo:
//
//	type RepositorioActivos interface {
//		Buscar(ctx context.Context, id string) (domain.Activo, error)
//	}
