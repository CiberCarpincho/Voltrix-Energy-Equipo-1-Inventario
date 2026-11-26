// Package httpapi es la capa de presentación (DAS 1.5): manejadores net/http que decodifican
// el JSON, aplican los límites de tamaño y de admisión, llaman a la capa de lógica de negocio
// (internal/servicio) y traducen sus resultados y errores a respuestas del contrato
// api/openapi.yaml. No conoce SQL ni el repositorio.
package httpapi

import (
	"log/slog"
	"net/http"

	"voltrix/inventario/internal/servicio"
)

// Dependencias son los casos de uso y parámetros que la presentación recibe ya construidos
// (los arma cmd/inventario).
type Dependencias struct {
	Salud *servicio.Salud
	// Contrato es el contenido de api/openapi.yaml, publicado en GET /openapi.yaml.
	Contrato []byte
	// Version que reporta /health.
	Version string
	// MaxConcurrentes es el límite del control de admisión (429).
	MaxConcurrentes int
	Log             *slog.Logger

	// Cada historia de usuario agrega aquí su caso de uso, por ejemplo:
	//   Activos *servicio.Activos
}

// Rutas de plataforma: no pasan por el control de admisión para que la salud y el contrato
// respondan aun con el servicio saturado.
var rutasSinAdmision = map[string]bool{"/health": true, "/openapi.yaml": true}

// Nuevo arma el enrutador con sus middlewares.
func Nuevo(d Dependencias) http.Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	mux := http.NewServeMux()
	registrarRutas(mux, d)

	// Cada middleware envuelve al anterior; la petición los recorre en orden inverso:
	// request id → log → recuperación de panic → control de admisión → ruta.
	var h http.Handler = mux
	h = controlarAdmision(h, d.MaxConcurrentes, rutasSinAdmision)
	h = recuperarPanico(h, d.Log)
	h = registrarPeticion(h, d.Log)
	h = asignarRequestID(h)
	return h
}

func registrarRutas(mux *http.ServeMux, d Dependencias) {
	// Plataforma
	mux.Handle("GET /health", manejarSalud(d.Salud, d.Version))
	mux.Handle("GET /openapi.yaml", manejarContrato(d.Contrato))

	// Operaciones de negocio: cada historia de usuario registra aquí sus rutas, por ejemplo:
	//   mux.Handle("GET /activos/{id}", manejarConsultarActivo(d.Activos))

	// Cualquier otra ruta responde 404 con el formato de error del contrato.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ResponderError(w, r, errRutaDesconocida)
	})
}

func manejarContrato(contrato []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(contrato)
	})
}
