package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"voltrix/inventario/app/business/domain"
)

// repositorioCatalogoFalso es un doble de prueba del puerto RepositorioCatalogo.
type repositorioCatalogoFalso struct {
	err         error
	llamadas    int
	plazoMaximo time.Duration
}

func (r *repositorioCatalogoFalso) registrar(ctx context.Context) {
	r.llamadas++
	if limite, ok := ctx.Deadline(); ok {
		r.plazoMaximo = time.Until(limite)
	}
}

func (r *repositorioCatalogoFalso) ListarPlantas(ctx context.Context, _ domain.FiltroPlantas, p domain.Paginacion) (domain.Pagina[domain.Planta], error) {
	r.registrar(ctx)
	return domain.Pagina[domain.Planta]{Paginacion: p}, r.err
}

func (r *repositorioCatalogoFalso) BuscarPlanta(ctx context.Context, _ string) (domain.DetallePlanta, error) {
	r.registrar(ctx)
	return domain.DetallePlanta{}, r.err
}

func (r *repositorioCatalogoFalso) ListarActivos(ctx context.Context, _ domain.FiltroActivos, p domain.Paginacion) (domain.Pagina[domain.Activo], error) {
	r.registrar(ctx)
	return domain.Pagina[domain.Activo]{Paginacion: p}, r.err
}

func (r *repositorioCatalogoFalso) BuscarActivo(ctx context.Context, _ string) (domain.Activo, error) {
	r.registrar(ctx)
	return domain.Activo{}, r.err
}

var paginaValida = domain.Paginacion{Limite: 50}

func TestCatalogoValidaAntesDeConsultar(t *testing.T) {
	repo := &repositorioCatalogoFalso{}
	catalogo := NuevoCatalogo(repo)
	ctx := context.Background()

	for _, err := range []error{
		func() error {
			_, err := catalogo.ListarPlantas(ctx, domain.FiltroPlantas{Tipo: "nuclear"}, paginaValida)
			return err
		}(),
		func() error {
			_, err := catalogo.ListarPlantas(ctx, domain.FiltroPlantas{ClienteID: "con espacio"}, paginaValida)
			return err
		}(),
		func() error {
			_, err := catalogo.ListarActivos(ctx, domain.FiltroActivos{Estado: "roto"}, paginaValida)
			return err
		}(),
		func() error {
			_, err := catalogo.ListarActivos(ctx, domain.FiltroActivos{}, domain.Paginacion{Limite: 0})
			return err
		}(),
		func() error {
			_, err := catalogo.ListarActivos(ctx, domain.FiltroActivos{}, domain.Paginacion{Limite: 201})
			return err
		}(),
		func() error {
			_, err := catalogo.ListarPlantas(ctx, domain.FiltroPlantas{}, domain.Paginacion{Limite: 10, Offset: -1})
			return err
		}(),
	} {
		if !errors.Is(err, domain.ErrSolicitudInvalida) {
			t.Errorf("se esperaba SolicitudInvalida, se obtuvo %v", err)
		}
	}
	if repo.llamadas != 0 {
		t.Fatalf("una consulta inválida no debe llegar al repositorio (%d llamadas)", repo.llamadas)
	}
}

func TestCatalogoConsultaConPlazo(t *testing.T) {
	repo := &repositorioCatalogoFalso{}
	pagina, err := NuevoCatalogo(repo).ListarActivos(context.Background(),
		domain.FiltroActivos{PlantaID: "PL-001", Tipo: "turbina", Estado: "operativo"}, domain.Paginacion{Limite: 200, Offset: 5})
	if err != nil || pagina.Limite != 200 || pagina.Offset != 5 {
		t.Fatalf("%+v, %v", pagina, err)
	}
	if repo.plazoMaximo <= 0 || repo.plazoMaximo > EsperaConsulta {
		t.Fatalf("la consulta debe tener un plazo de hasta %v, tuvo %v", EsperaConsulta, repo.plazoMaximo)
	}
}

func TestCatalogoMensajeNoEncontrado(t *testing.T) {
	catalogo := NuevoCatalogo(&repositorioCatalogoFalso{err: &domain.Error{Categoria: domain.ErrNoEncontrado}})
	_, err := catalogo.ConsultarActivo(context.Background(), "PL-999-PS-01")
	var errDominio *domain.Error
	if !errors.Is(err, domain.ErrNoEncontrado) || !errors.As(err, &errDominio) || errDominio.Mensaje != "El activo PL-999-PS-01 no existe." {
		t.Fatalf("activo: %v", err)
	}
	_, err = catalogo.ConsultarPlanta(context.Background(), "PL-999")
	if !errors.As(err, &errDominio) || errDominio.Mensaje != "La planta PL-999 no existe." {
		t.Fatalf("planta: %v", err)
	}

	caida := domain.NoDisponible(errors.New("sin conexión"))
	if _, err := NuevoCatalogo(&repositorioCatalogoFalso{err: caida}).ConsultarActivo(context.Background(), "x"); err != caida {
		t.Fatalf("los demás errores deben pasar sin cambios: %v", err)
	}
}
