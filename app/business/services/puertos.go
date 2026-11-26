// Package services es la capa de lógica de negocio: casos de uso del catálogo, validaciones de
// tipos, estados y orden temporal, y coordinación de los lotes con el pool de app/business/workers.
//
// No conoce HTTP ni SQL. Lo que necesita de la capa de acceso a datos lo declara como
// interfaces (puertos) en este archivo; app/infrastructure las implementa y
// app/main.go conecta una con otra. En las pruebas se reemplazan por dobles.
package services

import (
	"context"

	"voltrix/inventario/app/business/domain"
)

// VerificadorBaseDatos comprueba que la base de datos responde.
type VerificadorBaseDatos interface {
	Verificar(ctx context.Context) error
}

// RepositorioCatalogo consulta plantas y activos. Buscar devuelve un error de categoría
// domain.ErrNoEncontrado si el identificador no existe.
type RepositorioCatalogo interface {
	ListarPlantas(ctx context.Context, filtro domain.FiltroPlantas, pagina domain.Paginacion) (domain.Pagina[domain.Planta], error)
	BuscarPlanta(ctx context.Context, id string) (domain.DetallePlanta, error)
	ListarActivos(ctx context.Context, filtro domain.FiltroActivos, pagina domain.Paginacion) (domain.Pagina[domain.Activo], error)
	BuscarActivo(ctx context.Context, id string) (domain.Activo, error)
}

// Cada historia de usuario nueva agrega aquí el puerto de su repositorio.
