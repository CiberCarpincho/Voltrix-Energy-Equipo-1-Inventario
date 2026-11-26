package infraestructure

import (
	"database/sql"
	"errors"
	"testing"

	"voltrix/inventario/app/business/domain"
)

// Verifica la semilla del primer avance y las consultas del catálogo contra PostgreSQL, con las
// migraciones reales. Se omite si INVENTARIO_TEST_DSN no está definida (ver abrirPrueba).
func TestCatalogoConDatosDeEjemplo(t *testing.T) {
	db, ctx := abrirPrueba(t)
	if _, err := Migrar(ctx, db); err != nil {
		t.Fatal(err)
	}
	// Volver a migrar no duplica nada.
	if aplicadas, err := Migrar(ctx, db); err != nil || len(aplicadas) != 0 {
		t.Fatalf("segunda migración: %v, %v", aplicadas, err)
	}
	catalogo := Catalogo{DB: db}
	pagina1 := domain.Paginacion{Limite: 1}

	activo, err := catalogo.BuscarActivo(ctx, "PL-001-PS-07")
	if err != nil || activo.PlantaID != "PL-001" || activo.Estado != "operativo" || activo.CapacidadNominalKw != 0.55 ||
		activo.FechaInstalacion == nil || activo.FechaInstalacion.Format("2006-01-02") != "2025-03-14" {
		t.Fatalf("activo de ejemplo: %+v, %v", activo, err)
	}
	if _, err := catalogo.BuscarActivo(ctx, "PL-999-PS-01"); !errors.Is(err, domain.ErrNoEncontrado) || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("activo desconocido: %v", err)
	}

	for _, caso := range []struct {
		filtro domain.FiltroPlantas
		total  int
		id     string
	}{
		{domain.FiltroPlantas{}, 80, "PL-001"},
		{domain.FiltroPlantas{Tipo: "solar"}, 40, "PL-001"},
		{domain.FiltroPlantas{Tipo: "eolica"}, 25, "PL-002"},
		{domain.FiltroPlantas{Tipo: "hibrida"}, 15, "PL-003"},
		{domain.FiltroPlantas{ClienteID: "CLI-COOP-01"}, 2, "PL-001"},
	} {
		pagina, err := catalogo.ListarPlantas(ctx, caso.filtro, pagina1)
		if err != nil || pagina.Total != caso.total || len(pagina.Datos) != 1 || pagina.Datos[0].ID != caso.id {
			t.Fatalf("plantas %+v: %+v, %v", caso.filtro, pagina, err)
		}
	}

	for _, caso := range []struct {
		filtro domain.FiltroActivos
		pagina domain.Paginacion
		total  int
		id     string
	}{
		{domain.FiltroActivos{}, pagina1, 1270, "PL-001-BB-01"},
		{domain.FiltroActivos{PlantaID: "PL-001", Tipo: "panel_solar", Estado: "operativo"},
			domain.Paginacion{Limite: 1, Offset: 1}, 20, "PL-001-PS-02"},
		{domain.FiltroActivos{Estado: "en_mantenimiento"}, pagina1, 2, "PL-002-TB-01"},
	} {
		pagina, err := catalogo.ListarActivos(ctx, caso.filtro, caso.pagina)
		if err != nil || pagina.Total != caso.total || len(pagina.Datos) != 1 || pagina.Datos[0].ID != caso.id {
			t.Fatalf("activos %+v: %+v, %v", caso.filtro, pagina, err)
		}
	}

	// Un offset más allá del final devuelve una lista vacía con el total real.
	vacia, err := catalogo.ListarActivos(ctx, domain.FiltroActivos{PlantaID: "PL-001"}, domain.Paginacion{Limite: 10, Offset: 500})
	if err != nil || vacia.Total != 22 || vacia.Datos == nil || len(vacia.Datos) != 0 {
		t.Fatalf("página vacía: %+v, %v", vacia, err)
	}

	planta, err := catalogo.BuscarPlanta(ctx, "PL-003")
	if err != nil || planta.Municipio == nil || *planta.Municipio != "Dagua" || planta.Resumen.TotalActivos != 16 ||
		planta.Resumen.PorTipo["panel_solar"] != 10 || planta.Resumen.PorTipo["turbina"] != 2 ||
		planta.Resumen.PorTipo["banco_baterias"] != 4 || planta.Resumen.PorEstado["en_mantenimiento"] != 1 {
		t.Fatalf("planta de ejemplo: %+v, %v", planta, err)
	}
	if _, err := catalogo.BuscarPlanta(ctx, "PL-999"); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("planta desconocida: %v", err)
	}
}
