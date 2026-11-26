package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"voltrix/inventario/app/business/domain"
)

// TamanoMaximoCuerpo limita el JSON de una petición (1 MiB).
const TamanoMaximoCuerpo = 1 << 20

var errRutaDesconocida = domain.NoEncontrado("La ruta no existe")

// cuerpoError es el esquema Error del contrato.
type cuerpoError struct {
	Codigo    string         `json:"codigo"`
	Mensaje   string         `json:"mensaje"`
	RequestID string         `json:"requestId"`
	Detalles  []detalleError `json:"detalles,omitempty"`
}

type detalleError struct {
	Campo    string `json:"campo"`
	Problema string `json:"problema"`
}

// Respuesta por defecto de cada categoría de error del dominio.
var respuestasPorCategoria = []struct {
	categoria error
	status    int
	codigo    string
	mensaje   string
}{
	{domain.ErrSolicitudInvalida, http.StatusBadRequest, "SOLICITUD_INVALIDA", "La solicitud no es válida."},
	{domain.ErrNoEncontrado, http.StatusNotFound, "NO_ENCONTRADO", "El recurso no existe."},
	{domain.ErrDuplicado, http.StatusConflict, "DUPLICADO", "El recurso ya existe."},
	{domain.ErrNoDisponible, http.StatusServiceUnavailable, "NO_DISPONIBLE", "No hay conexión con la base de datos."},
}

// EscribirJSON responde con el valor serializado como JSON.
func EscribirJSON(w http.ResponseWriter, status int, valor any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(valor)
}

// ResponderError traduce un error de las capas de abajo a la respuesta del contrato. Es el
// único lugar donde se decide el código HTTP de un error; un error sin categoría se registra
// en el log y se responde como 500 ERROR_INTERNO sin exponer el detalle.
func ResponderError(w http.ResponseWriter, r *http.Request, err error) {
	status, cuerpo := http.StatusInternalServerError, cuerpoError{
		Codigo:  "ERROR_INTERNO",
		Mensaje: "Ocurrió un error inesperado.",
	}
	for _, respuesta := range respuestasPorCategoria {
		if errors.Is(err, respuesta.categoria) {
			status, cuerpo.Codigo, cuerpo.Mensaje = respuesta.status, respuesta.codigo, respuesta.mensaje
			break
		}
	}

	var errDominio *domain.Error
	if errors.As(err, &errDominio) {
		if errDominio.Codigo != "" {
			cuerpo.Codigo = errDominio.Codigo
		}
		if errDominio.Mensaje != "" && status != http.StatusInternalServerError {
			cuerpo.Mensaje = errDominio.Mensaje
		}
		for _, d := range errDominio.Detalles {
			cuerpo.Detalles = append(cuerpo.Detalles, detalleError{Campo: d.Campo, Problema: d.Problema})
		}
	}

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "error al atender la petición",
			"error", err, "requestId", RequestID(r.Context()))
	}
	cuerpo.RequestID = RequestID(r.Context())
	EscribirJSON(w, status, cuerpo)
}

// LeerJSON decodifica el cuerpo de la petición en destino. Rechaza cuerpos de más de
// TamanoMaximoCuerpo, campos que no existen en destino y contenido después del objeto.
// Devuelve un error de categoría SolicitudInvalida listo para ResponderError.
func LeerJSON(w http.ResponseWriter, r *http.Request, destino any) error {
	decodificador := json.NewDecoder(http.MaxBytesReader(w, r.Body, TamanoMaximoCuerpo))
	decodificador.DisallowUnknownFields()
	if err := decodificador.Decode(destino); err != nil {
		var demasiadoGrande *http.MaxBytesError
		if errors.As(err, &demasiadoGrande) {
			return domain.Invalido("El cuerpo supera el tamaño máximo permitido.")
		}
		return domain.Invalido("El cuerpo no es un JSON válido para esta operación.",
			domain.Detalle{Campo: "cuerpo", Problema: err.Error()})
	}
	if decodificador.More() {
		return domain.Invalido("El cuerpo debe contener un único objeto JSON.")
	}
	return nil
}
