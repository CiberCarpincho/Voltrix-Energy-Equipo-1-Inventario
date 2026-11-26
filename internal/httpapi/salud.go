package httpapi

import (
	"net/http"
	"time"

	"voltrix/inventario/internal/servicio"
)

// respuestaSalud es el esquema Salud del contrato.
type respuestaSalud struct {
	Estado          string `json:"estado"`
	BaseDatos       string `json:"baseDatos"`
	Version         string `json:"version"`
	TiempoActivoSeg int64  `json:"tiempoActivoSeg"`
}

// manejarSalud responde 200 si PostgreSQL contesta y 503 si no.
func manejarSalud(salud *servicio.Salud, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		estado := salud.Consultar(r.Context())
		cuerpo := respuestaSalud{
			Estado:          "disponible",
			BaseDatos:       "ok",
			Version:         version,
			TiempoActivoSeg: int64(estado.TiempoActivo / time.Second),
		}
		codigo := http.StatusOK
		if !estado.BaseDatosDisponible {
			cuerpo.Estado, cuerpo.BaseDatos = "no_disponible", "error"
			codigo = http.StatusServiceUnavailable
		}
		EscribirJSON(w, codigo, cuerpo)
	})
}
