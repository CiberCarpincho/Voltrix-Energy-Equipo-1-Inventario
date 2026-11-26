package services

import (
	"context"
	"errors"
	"time"

	"voltrix/inventario/app/business/domain"
)

// EsperaConsulta es el tiempo máximo de una consulta al catálogo.
const EsperaConsulta = 3 * time.Second

// Catalogo es el caso de uso de consulta de plantas y activos (datos de ejemplo del primer
// avance).
type Catalogo struct {
	repositorio RepositorioCatalogo
}

// NuevoCatalogo crea el caso de uso sobre el repositorio dado.
func NuevoCatalogo(repositorio RepositorioCatalogo) *Catalogo {
	return &Catalogo{repositorio: repositorio}
}

// ListarPlantas devuelve una página de plantas, filtradas por tipo y cliente.
func (c *Catalogo) ListarPlantas(ctx context.Context, filtro domain.FiltroPlantas, pagina domain.Paginacion) (domain.Pagina[domain.Planta], error) {
	if !domain.ValorPermitido(filtro.Tipo, domain.TiposPlanta) ||
		(filtro.ClienteID != "" && !domain.IdentificadorValido(filtro.ClienteID)) {
		return domain.Pagina[domain.Planta]{}, domain.Invalido("Filtro de plantas inválido")
	}
	if err := validarPaginacion(pagina); err != nil {
		return domain.Pagina[domain.Planta]{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, EsperaConsulta)
	defer cancel()
	return c.repositorio.ListarPlantas(ctx, filtro, pagina)
}

// ConsultarPlanta devuelve una planta con el resumen de sus activos.
func (c *Catalogo) ConsultarPlanta(ctx context.Context, id string) (domain.DetallePlanta, error) {
	ctx, cancel := context.WithTimeout(ctx, EsperaConsulta)
	defer cancel()
	planta, err := c.repositorio.BuscarPlanta(ctx, id)
	return planta, conMensajeNoEncontrado(err, "La planta "+id+" no existe.")
}

// ListarActivos devuelve una página de activos, filtrados por planta, tipo y estado.
func (c *Catalogo) ListarActivos(ctx context.Context, filtro domain.FiltroActivos, pagina domain.Paginacion) (domain.Pagina[domain.Activo], error) {
	if (filtro.PlantaID != "" && !domain.IdentificadorValido(filtro.PlantaID)) ||
		!domain.ValorPermitido(filtro.Tipo, domain.TiposActivo) ||
		!domain.ValorPermitido(filtro.Estado, domain.EstadosActivo) {
		return domain.Pagina[domain.Activo]{}, domain.Invalido("Filtro de activos inválido")
	}
	if err := validarPaginacion(pagina); err != nil {
		return domain.Pagina[domain.Activo]{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, EsperaConsulta)
	defer cancel()
	return c.repositorio.ListarActivos(ctx, filtro, pagina)
}

// ConsultarActivo devuelve un activo por su identificador.
func (c *Catalogo) ConsultarActivo(ctx context.Context, id string) (domain.Activo, error) {
	ctx, cancel := context.WithTimeout(ctx, EsperaConsulta)
	defer cancel()
	activo, err := c.repositorio.BuscarActivo(ctx, id)
	return activo, conMensajeNoEncontrado(err, "El activo "+id+" no existe.")
}

// validarPaginacion comprueba los rangos de limit y offset del contrato.
func validarPaginacion(pagina domain.Paginacion) error {
	if pagina.Limite < 1 || pagina.Limite > domain.LimiteMaximo {
		return domain.Invalido("limit debe estar entre 1 y 200")
	}
	if pagina.Offset < 0 {
		return domain.Invalido("offset debe ser un entero no negativo")
	}
	return nil
}

// conMensajeNoEncontrado agrega el mensaje para el consumidor a un error NoEncontrado del
// repositorio; los demás errores pasan sin cambios.
func conMensajeNoEncontrado(err error, mensaje string) error {
	if errors.Is(err, domain.ErrNoEncontrado) {
		return &domain.Error{Categoria: domain.ErrNoEncontrado, Mensaje: mensaje, Causa: err}
	}
	return err
}
