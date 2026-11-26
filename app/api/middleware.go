package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

type claveRequestID struct{}

var formatoUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// RequestID devuelve el identificador de la petición en curso (X-Request-Id).
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(claveRequestID{}).(string)
	return id
}

// asignarRequestID reutiliza el X-Request-Id del consumidor si es un UUID (para seguir la
// llamada entre servicios) o genera uno nuevo, y lo devuelve en la respuesta.
func asignarRequestID(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !formatoUUID.MatchString(id) {
			id = nuevoUUID()
		}
		w.Header().Set("X-Request-Id", id)
		siguiente.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claveRequestID{}, id)))
	})
}

// nuevoUUID genera un UUID versión 4.
func nuevoUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read no devuelve error desde Go 1.24
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// respuestaRegistrada guarda el código HTTP para el log.
type respuestaRegistrada struct {
	http.ResponseWriter
	status int
}

func (w *respuestaRegistrada) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *respuestaRegistrada) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap permite a http.ResponseController llegar al ResponseWriter original.
func (w *respuestaRegistrada) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// registrarPeticion escribe una línea de log por petición con la duración, la ruta, el código
// HTTP, el identificador de la petición y el equipo consumidor (DAS, trazabilidad).
func registrarPeticion(siguiente http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		registrada := &respuestaRegistrada{ResponseWriter: w}
		siguiente.ServeHTTP(registrada, r)
		if registrada.status == 0 {
			registrada.status = http.StatusOK
		}
		log.InfoContext(r.Context(), "petición",
			"metodo", r.Method,
			"ruta", r.URL.Path,
			"status", registrada.status,
			"duracionMs", float64(time.Since(inicio).Microseconds())/1000,
			"requestId", RequestID(r.Context()),
			"equipoConsumidor", r.Header.Get("X-Equipo-Consumidor"),
		)
	})
}

// recuperarPanico convierte un panic en un 500 con el formato del contrato, sin tumbar el
// proceso.
func recuperarPanico(siguiente http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if valor := recover(); valor != nil {
				if valor == http.ErrAbortHandler {
					panic(valor)
				}
				log.ErrorContext(r.Context(), "panic al atender la petición",
					"panic", fmt.Sprint(valor), "requestId", RequestID(r.Context()))
				ResponderError(w, r, fmt.Errorf("panic: %v", valor))
			}
		}()
		siguiente.ServeHTTP(w, r)
	})
}

// controlarAdmision limita las peticiones de negocio atendidas a la vez (DAS, control de
// admisión). Cuando no hay cupo responde de inmediato 429 con Retry-After, en lugar de
// encolar trabajo que la EC2 de 1 GiB no puede sostener.
func controlarAdmision(siguiente http.Handler, maximo int, excluidas map[string]bool) http.Handler {
	cupos := make(chan struct{}, maximo)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if excluidas[r.URL.Path] {
			siguiente.ServeHTTP(w, r)
			return
		}
		select {
		case cupos <- struct{}{}:
			defer func() { <-cupos }()
			siguiente.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			EscribirJSON(w, http.StatusTooManyRequests, cuerpoError{
				Codigo:    "SOBRECARGA",
				Mensaje:   "El servicio está atendiendo el máximo de peticiones simultáneas.",
				RequestID: RequestID(r.Context()),
			})
		}
	})
}
