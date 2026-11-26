package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"voltrix/inventario/app/business/domain"
)

// catalogoFalso es un doble de prueba de services.RepositorioCatalogo.
type catalogoFalso struct {
	err           error
	llamadas      int
	filtroPlantas domain.FiltroPlantas
	filtroActivos domain.FiltroActivos
	pagina        domain.Paginacion
}

var (
	creado     = time.Date(2026, 11, 19, 13, 0, 0, 0, time.UTC)
	instalado  = time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC)
	jamundi    = "Jamundí"
	plantaPL01 = domain.Planta{ID: "PL-001", Nombre: "Microplanta Solar Jamundí", Tipo: "solar",
		ClienteID: "CLI-COOP-01", Municipio: &jamundi, CapacidadInstaladaKw: 120.5, CreadoEn: creado}
	activoPS07 = domain.Activo{ID: "PL-001-PS-07", PlantaID: "PL-001", Tipo: "panel_solar", Estado: "operativo",
		CapacidadNominalKw: 0.55, FechaInstalacion: &instalado, ActualizadoEn: creado}
)

func (c *catalogoFalso) ListarPlantas(_ context.Context, f domain.FiltroPlantas, p domain.Paginacion) (domain.Pagina[domain.Planta], error) {
	c.llamadas++
	c.filtroPlantas, c.pagina = f, p
	sinMunicipio := plantaPL01
	sinMunicipio.ID, sinMunicipio.Municipio = "PL-004", nil
	return domain.Pagina[domain.Planta]{Datos: []domain.Planta{plantaPL01, sinMunicipio}, Total: 40, Paginacion: p}, c.err
}

func (c *catalogoFalso) BuscarPlanta(_ context.Context, id string) (domain.DetallePlanta, error) {
	c.llamadas++
	if c.err != nil {
		return domain.DetallePlanta{}, c.err
	}
	if id != "PL-001" {
		return domain.DetallePlanta{}, &domain.Error{Categoria: domain.ErrNoEncontrado}
	}
	return domain.DetallePlanta{Planta: plantaPL01, Resumen: domain.ResumenActivos{
		TotalActivos: 22,
		PorTipo:      map[string]int{"panel_solar": 20, "banco_baterias": 2},
		PorEstado:    map[string]int{"operativo": 22},
	}}, nil
}

func (c *catalogoFalso) ListarActivos(_ context.Context, f domain.FiltroActivos, p domain.Paginacion) (domain.Pagina[domain.Activo], error) {
	c.llamadas++
	c.filtroActivos, c.pagina = f, p
	return domain.Pagina[domain.Activo]{Datos: []domain.Activo{activoPS07}, Total: 1270, Paginacion: p}, c.err
}

func (c *catalogoFalso) BuscarActivo(_ context.Context, id string) (domain.Activo, error) {
	c.llamadas++
	if c.err != nil {
		return domain.Activo{}, c.err
	}
	if id != "PL-001-PS-07" {
		return domain.Activo{}, &domain.Error{Categoria: domain.ErrNoEncontrado}
	}
	return activoPS07, nil
}

// jsonIgual compara el cuerpo con el JSON esperado sin importar el orden de los campos.
func jsonIgual(t *testing.T, cuerpo []byte, esperado string) {
	t.Helper()
	var obtenido, quiere any
	if err := json.Unmarshal(cuerpo, &obtenido); err != nil {
		t.Fatalf("respuesta no es JSON: %s", cuerpo)
	}
	if err := json.Unmarshal([]byte(esperado), &quiere); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(obtenido)
	b, _ := json.Marshal(quiere)
	if string(a) != string(b) {
		t.Fatalf("JSON distinto:\nobtenido %s\nesperado %s", a, b)
	}
}

func TestConsultarActivo(t *testing.T) {
	w := pedir(nuevoServidor(t, nil, nil), "GET", "/activos/PL-001-PS-07", "")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d %s", w.Code, w.Body)
	}
	jsonIgual(t, w.Body.Bytes(), `{"id":"PL-001-PS-07","plantaId":"PL-001","tipo":"panel_solar","estado":"operativo",
		"capacidadNominalKw":0.55,"fechaInstalacion":"2025-03-14","actualizadoEn":"2026-11-19T13:00:00Z"}`)
}

func TestConsultarActivoDesconocido(t *testing.T) {
	w := pedir(nuevoServidor(t, nil, nil), "GET", "/activos/PL-999-PS-01", "")
	cuerpo := decodificar[cuerpoError](t, w)
	if w.Code != http.StatusNotFound || cuerpo.Codigo != "NO_ENCONTRADO" || cuerpo.Mensaje != "El activo PL-999-PS-01 no existe." {
		t.Fatalf("HTTP %d %+v", w.Code, cuerpo)
	}
}

func TestConsultarPlantaConResumen(t *testing.T) {
	w := pedir(nuevoServidor(t, nil, nil), "GET", "/plantas/PL-001", "")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d %s", w.Code, w.Body)
	}
	jsonIgual(t, w.Body.Bytes(), `{"id":"PL-001","nombre":"Microplanta Solar Jamundí","tipo":"solar","clienteId":"CLI-COOP-01",
		"municipio":"Jamundí","capacidadInstaladaKw":120.5,"creadoEn":"2026-11-19T13:00:00Z",
		"resumen":{"totalActivos":22,"porTipo":{"panel_solar":20,"turbina":0,"banco_baterias":2},
		"porEstado":{"operativo":22,"en_mantenimiento":0,"fuera_de_servicio":0}}}`)

	if w := pedir(nuevoServidor(t, nil, nil), "GET", "/plantas/PL-999", ""); w.Code != http.StatusNotFound {
		t.Fatalf("planta desconocida: HTTP %d", w.Code)
	}
}

func TestListarPlantas(t *testing.T) {
	repo := &catalogoFalso{}
	w := pedir(nuevoServidor(t, nil, repo), "GET", "/plantas?tipo=solar&clienteId=CLI-COOP-01&limit=2&offset=1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d %s", w.Code, w.Body)
	}
	if repo.filtroPlantas != (domain.FiltroPlantas{Tipo: "solar", ClienteID: "CLI-COOP-01"}) ||
		repo.pagina != (domain.Paginacion{Limite: 2, Offset: 1}) {
		t.Fatalf("filtro %+v, paginación %+v", repo.filtroPlantas, repo.pagina)
	}
	var pagina struct {
		Datos         []map[string]any
		Total         int
		Limit, Offset int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pagina); err != nil {
		t.Fatal(err)
	}
	if pagina.Total != 40 || pagina.Limit != 2 || pagina.Offset != 1 || len(pagina.Datos) != 2 {
		t.Fatalf("página: %+v", pagina)
	}
	if _, tiene := pagina.Datos[1]["municipio"]; tiene {
		t.Fatal("un municipio nulo debe omitirse")
	}
}

func TestListarActivosPaginacionPorDefecto(t *testing.T) {
	repo := &catalogoFalso{}
	w := pedir(nuevoServidor(t, nil, repo), "GET", "/activos?plantaId=PL-001&tipo=panel_solar&estado=operativo", "")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d %s", w.Code, w.Body)
	}
	if repo.filtroActivos != (domain.FiltroActivos{PlantaID: "PL-001", Tipo: "panel_solar", Estado: "operativo"}) ||
		repo.pagina != (domain.Paginacion{Limite: 50, Offset: 0}) {
		t.Fatalf("filtro %+v, paginación %+v", repo.filtroActivos, repo.pagina)
	}
	if !strings.Contains(w.Body.String(), `"total":1270`) {
		t.Fatalf("cuerpo: %s", w.Body)
	}
}

func TestCatalogoRechazaConsultasInvalidas(t *testing.T) {
	for ruta, mensaje := range map[string]string{
		"/plantas?tipo=nuclear":          "Filtro de plantas inválido",
		"/plantas?clienteId=a%20b":       "Filtro de plantas inválido",
		"/activos?estado=roto":           "Filtro de activos inválido",
		"/activos?plantaId=..%2F":        "Filtro de activos inválido",
		"/activos?limit=0":               "limit debe estar entre 1 y 200",
		"/activos?limit=201":             "limit debe estar entre 1 y 200",
		"/plantas?limit=abc":             "limit debe estar entre 1 y 200",
		"/activos?offset=-1":             "offset debe ser un entero no negativo",
		"/plantas?offset=uno":            "offset debe ser un entero no negativo",
		"/activos?tipo=turbina&limit=0":  "limit debe estar entre 1 y 200",
		"/activos?tipo=molino&offset=-1": "Filtro de activos inválido",
	} {
		repo := &catalogoFalso{}
		w := pedir(nuevoServidor(t, nil, repo), "GET", ruta, "")
		cuerpo := decodificar[cuerpoError](t, w)
		if w.Code != http.StatusBadRequest || cuerpo.Codigo != "SOLICITUD_INVALIDA" || cuerpo.Mensaje != mensaje {
			t.Errorf("%s: HTTP %d %+v", ruta, w.Code, cuerpo)
		}
		if repo.llamadas != 0 {
			t.Errorf("%s: no debía consultar el repositorio", ruta)
		}
	}
}

func TestCatalogoSinBaseDatos(t *testing.T) {
	repo := &catalogoFalso{err: domain.NoDisponible(errors.New("conexión rechazada"))}
	for _, ruta := range []string{"/plantas", "/plantas/PL-001", "/activos", "/activos/PL-001-PS-07"} {
		w := pedir(nuevoServidor(t, nil, repo), "GET", ruta, "")
		if cuerpo := decodificar[cuerpoError](t, w); w.Code != http.StatusServiceUnavailable || cuerpo.Codigo != "NO_DISPONIBLE" {
			t.Errorf("%s: HTTP %d %+v", ruta, w.Code, cuerpo)
		}
	}
}

func TestPostDeDemostracion(t *testing.T) {
	servidor := nuevoServidor(t, nil, nil)
	w := pedir(servidor, "POST", "/activos/consulta", `{"ids":["PL-001-PS-07","PL-002-TB-01","PL-999-PS-01"]}`)
	var lote struct {
		Encontrados  []struct{ ID string }
		Desconocidos []string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lote); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || w.Header().Get("X-Inventario-Demo") != "true" ||
		len(lote.Encontrados) != 2 || lote.Encontrados[0].ID != "PL-001-PS-07" ||
		len(lote.Desconocidos) != 1 || lote.Desconocidos[0] != "PL-999-PS-01" {
		t.Fatalf("consulta de ejemplo: HTTP %d %s", w.Code, w.Body)
	}

	w = pedir(servidor, "POST", "/activos", `{"id":"otro"}`)
	if cuerpo := decodificar[cuerpoError](t, w); w.Code != http.StatusBadRequest || cuerpo.Codigo != "SOLICITUD_INVALIDA" ||
		w.Header().Get("X-Inventario-Demo") != "true" {
		t.Fatalf("cuerpo distinto al ejemplo: HTTP %d %+v", w.Code, cuerpo)
	}
}
