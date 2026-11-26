package domain

import (
	"regexp"
	"slices"
	"time"
)

// Tipos de planta, tipos y estados de activo del contrato (TipoPlanta, TipoActivo, EstadoActivo).
var (
	TiposPlanta   = []string{"solar", "eolica", "hibrida"}
	TiposActivo   = []string{"panel_solar", "turbina", "banco_baterias"}
	EstadosActivo = []string{"operativo", "en_mantenimiento", "fuera_de_servicio"}
)

// Límites de paginación del contrato (parámetros limit y offset).
const (
	LimitePorDefecto = 50
	LimiteMaximo     = 200
)

var formatoIdentificador = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// IdentificadorValido indica si el texto cumple el esquema Identificador del contrato.
func IdentificadorValido(id string) bool {
	return formatoIdentificador.MatchString(id)
}

// ValorPermitido indica si valor está vacío (filtro no aplicado) o es una de las opciones.
func ValorPermitido(valor string, opciones []string) bool {
	return valor == "" || slices.Contains(opciones, valor)
}

// Planta es una microplanta del catálogo.
type Planta struct {
	ID                   string
	Nombre               string
	Tipo                 string
	ClienteID            string
	Municipio            *string // opcional
	CapacidadInstaladaKw float64
	CreadoEn             time.Time
}

// ResumenActivos cuenta los activos de una planta por tipo y por estado.
type ResumenActivos struct {
	TotalActivos int
	PorTipo      map[string]int // claves: TiposActivo
	PorEstado    map[string]int // claves: EstadosActivo
}

// DetallePlanta es una planta con el resumen de sus activos.
type DetallePlanta struct {
	Planta
	Resumen ResumenActivos
}

// Activo es un equipo físico de una planta.
type Activo struct {
	ID                 string
	PlantaID           string
	Tipo               string
	Estado             string
	CapacidadNominalKw float64
	FechaInstalacion   *time.Time // opcional; solo fecha
	ActualizadoEn      time.Time
}

// FiltroPlantas restringe el listado de plantas; un campo vacío no filtra.
type FiltroPlantas struct {
	Tipo      string
	ClienteID string
}

// FiltroActivos restringe el listado de activos; un campo vacío no filtra.
type FiltroActivos struct {
	PlantaID string
	Tipo     string
	Estado   string
}

// Paginacion indica cuántos elementos devolver y cuántos omitir.
type Paginacion struct {
	Limite int
	Offset int
}

// Pagina es un fragmento de un listado con el total de elementos que cumplen el filtro.
type Pagina[T any] struct {
	Datos []T
	Total int
	Paginacion
}
