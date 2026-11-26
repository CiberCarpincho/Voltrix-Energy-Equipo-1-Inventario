// Package api es la capa de presentación (DAS 1.5): manejadores net/http que decodifican
// el JSON, aplican los límites de tamaño y de admisión, llaman a la capa de lógica de negocio
// (app/business/services) y traducen sus resultados y errores a respuestas del contrato
// docs/openapi.yaml. No conoce SQL ni app/infraestructure.
package api

import (
	"log/slog"
	"net/http"

	"voltrix/inventario/app/business/services"
)

// Dependencias son los casos de uso y parámetros que la presentación recibe ya construidos
// (los arma app/main.go).
type Dependencias struct {
	Salud    *services.Salud
	Catalogo *services.Catalogo
	// Contrato es el contenido de docs/openapi.yaml, publicado en GET /openapi.yaml y fuente
	// de las respuestas de demostración de los POST.
	Contrato []byte
	// Version que reporta /health.
	Version string
	// MaxConcurrentes es el límite del control de admisión (429).
	MaxConcurrentes int
	Log             *slog.Logger

	// Cada historia de usuario agrega aquí su caso de uso.
}

// Rutas de plataforma: no pasan por el control de admisión para que la salud y el contrato
// respondan aun con el servicio saturado.
var rutasSinAdmision = map[string]bool{"/health": true, "/openapi.yaml": true}

// Nuevo arma el enrutador con sus middlewares. Falla si el contrato no trae los ejemplos de
// los POST de demostración.
func Nuevo(d Dependencias) (http.Handler, error) {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	mux := http.NewServeMux()
	if err := registrarRutas(mux, d); err != nil {
		return nil, err
	}

	// Cada middleware envuelve al anterior; la petición los recorre en orden inverso:
	// request id → log → recuperación de panic → control de admisión → ruta.
	var h http.Handler = mux
	h = controlarAdmision(h, d.MaxConcurrentes, rutasSinAdmision)
	h = recuperarPanico(h, d.Log)
	h = registrarPeticion(h, d.Log)
	h = asignarRequestID(h)
	return h, nil
}

func registrarRutas(mux *http.ServeMux, d Dependencias) error {
	// Plataforma
	mux.Handle("GET /health", manejarSalud(d.Salud, d.Version))
	mux.Handle("GET /openapi.yaml", manejarContrato(d.Contrato))

	// Catálogo con los datos de ejemplo del primer avance
	mux.Handle("GET /plantas", manejarListarPlantas(d.Catalogo))
	mux.Handle("GET /plantas/{plantaId}", manejarConsultarPlanta(d.Catalogo))
	mux.Handle("GET /activos", manejarListarActivos(d.Catalogo))
	mux.Handle("GET /activos/{id}", manejarConsultarActivo(d.Catalogo))

	// POST del contrato con respuestas de demostración (ejemplos.go). Cada historia de usuario
	// registra aquí su operación real.
	if err := registrarEjemplos(mux, d.Contrato); err != nil {
		return err
	}

	// Cualquier otra ruta responde 404 con el formato de error del contrato.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ResponderError(w, r, errRutaDesconocida)
	})
	return nil
}

func manejarContrato(contrato []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(contrato)
	})
}
