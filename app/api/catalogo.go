package api

import (
	"net/http"
	"strconv"
	"time"

	"voltrix/inventario/app/business/domain"
	"voltrix/inventario/app/business/services"
)

// DTO con los esquemas Planta, PlantaDetalle, Activo y Pagina* del contrato.

type plantaDTO struct {
	ID                   string  `json:"id"`
	Nombre               string  `json:"nombre"`
	Tipo                 string  `json:"tipo"`
	ClienteID            string  `json:"clienteId"`
	Municipio            *string `json:"municipio,omitempty"`
	CapacidadInstaladaKw float64 `json:"capacidadInstaladaKw"`
	CreadoEn             string  `json:"creadoEn"`
}

type plantaDetalleDTO struct {
	plantaDTO
	Resumen resumenDTO `json:"resumen"`
}

type resumenDTO struct {
	TotalActivos int `json:"totalActivos"`
	PorTipo      struct {
		PanelSolar    int `json:"panel_solar"`
		Turbina       int `json:"turbina"`
		BancoBaterias int `json:"banco_baterias"`
	} `json:"porTipo"`
	PorEstado struct {
		Operativo       int `json:"operativo"`
		EnMantenimiento int `json:"en_mantenimiento"`
		FueraDeServicio int `json:"fuera_de_servicio"`
	} `json:"porEstado"`
}

type activoDTO struct {
	ID                 string  `json:"id"`
	PlantaID           string  `json:"plantaId"`
	Tipo               string  `json:"tipo"`
	Estado             string  `json:"estado"`
	CapacidadNominalKw float64 `json:"capacidadNominalKw"`
	FechaInstalacion   *string `json:"fechaInstalacion,omitempty"`
	ActualizadoEn      string  `json:"actualizadoEn"`
}

type paginaDTO[T any] struct {
	Datos  []T `json:"datos"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// fechaHora usa RFC 3339 en UTC (format: date-time del contrato).
func fechaHora(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func nuevaPlantaDTO(p domain.Planta) plantaDTO {
	return plantaDTO{
		ID: p.ID, Nombre: p.Nombre, Tipo: p.Tipo, ClienteID: p.ClienteID, Municipio: p.Municipio,
		CapacidadInstaladaKw: p.CapacidadInstaladaKw, CreadoEn: fechaHora(p.CreadoEn),
	}
}

func nuevaPlantaDetalleDTO(d domain.DetallePlanta) plantaDetalleDTO {
	dto := plantaDetalleDTO{plantaDTO: nuevaPlantaDTO(d.Planta)}
	dto.Resumen.TotalActivos = d.Resumen.TotalActivos
	dto.Resumen.PorTipo.PanelSolar = d.Resumen.PorTipo["panel_solar"]
	dto.Resumen.PorTipo.Turbina = d.Resumen.PorTipo["turbina"]
	dto.Resumen.PorTipo.BancoBaterias = d.Resumen.PorTipo["banco_baterias"]
	dto.Resumen.PorEstado.Operativo = d.Resumen.PorEstado["operativo"]
	dto.Resumen.PorEstado.EnMantenimiento = d.Resumen.PorEstado["en_mantenimiento"]
	dto.Resumen.PorEstado.FueraDeServicio = d.Resumen.PorEstado["fuera_de_servicio"]
	return dto
}

func nuevoActivoDTO(a domain.Activo) activoDTO {
	dto := activoDTO{
		ID: a.ID, PlantaID: a.PlantaID, Tipo: a.Tipo, Estado: a.Estado,
		CapacidadNominalKw: a.CapacidadNominalKw, ActualizadoEn: fechaHora(a.ActualizadoEn),
	}
	if a.FechaInstalacion != nil {
		fecha := a.FechaInstalacion.Format(time.DateOnly)
		dto.FechaInstalacion = &fecha
	}
	return dto
}

func nuevaPaginaDTO[T, D any](p domain.Pagina[T], convertir func(T) D) paginaDTO[D] {
	dto := paginaDTO[D]{Datos: make([]D, 0, len(p.Datos)), Total: p.Total, Limit: p.Limite, Offset: p.Offset}
	for _, elemento := range p.Datos {
		dto.Datos = append(dto.Datos, convertir(elemento))
	}
	return dto
}

// leerPaginacion lee limit y offset de la consulta. Un valor que no es entero se convierte
// en uno fuera de rango para que el caso de uso responda con el mensaje del contrato.
func leerPaginacion(r *http.Request) domain.Paginacion {
	pagina := domain.Paginacion{Limite: domain.LimitePorDefecto}
	q := r.URL.Query()
	if q.Has("limit") {
		limite, err := strconv.Atoi(q.Get("limit"))
		if err != nil {
			limite = 0
		}
		pagina.Limite = limite
	}
	if q.Has("offset") {
		offset, err := strconv.Atoi(q.Get("offset"))
		if err != nil {
			offset = -1
		}
		pagina.Offset = offset
	}
	return pagina
}

func manejarListarPlantas(catalogo *services.Catalogo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		pagina, err := catalogo.ListarPlantas(r.Context(),
			domain.FiltroPlantas{Tipo: q.Get("tipo"), ClienteID: q.Get("clienteId")}, leerPaginacion(r))
		if err != nil {
			ResponderError(w, r, err)
			return
		}
		EscribirJSON(w, http.StatusOK, nuevaPaginaDTO(pagina, nuevaPlantaDTO))
	}
}

func manejarConsultarPlanta(catalogo *services.Catalogo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		planta, err := catalogo.ConsultarPlanta(r.Context(), r.PathValue("plantaId"))
		if err != nil {
			ResponderError(w, r, err)
			return
		}
		EscribirJSON(w, http.StatusOK, nuevaPlantaDetalleDTO(planta))
	}
}

func manejarListarActivos(catalogo *services.Catalogo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filtro := domain.FiltroActivos{PlantaID: q.Get("plantaId"), Tipo: q.Get("tipo"), Estado: q.Get("estado")}
		pagina, err := catalogo.ListarActivos(r.Context(), filtro, leerPaginacion(r))
		if err != nil {
			ResponderError(w, r, err)
			return
		}
		EscribirJSON(w, http.StatusOK, nuevaPaginaDTO(pagina, nuevoActivoDTO))
	}
}

func manejarConsultarActivo(catalogo *services.Catalogo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		activo, err := catalogo.ConsultarActivo(r.Context(), r.PathValue("id"))
		if err != nil {
			ResponderError(w, r, err)
			return
		}
		EscribirJSON(w, http.StatusOK, nuevoActivoDTO(activo))
	}
}
