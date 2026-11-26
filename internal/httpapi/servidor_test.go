package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"voltrix/inventario/internal/dominio"
	"voltrix/inventario/internal/servicio"
)

var logSilencioso = slog.New(slog.DiscardHandler)

// baseFalsa es un doble de prueba del puerto servicio.VerificadorBaseDatos.
type baseFalsa struct{ err error }

func (b baseFalsa) Verificar(context.Context) error { return b.err }

func nuevoServidor(errBase error) http.Handler {
	return Nuevo(Dependencias{
		Salud:           servicio.NuevaSalud(baseFalsa{err: errBase}),
		Contrato:        []byte("openapi: 3.0.3\n"),
		Version:         "prueba",
		MaxConcurrentes: 2,
		Log:             logSilencioso,
	})
}

func pedir(h http.Handler, metodo, ruta string, cuerpo string, cabeceras ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	for i := 0; i+1 < len(cabeceras); i += 2 {
		r.Header.Set(cabeceras[i], cabeceras[i+1])
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func decodificar[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var valor T
	if err := json.Unmarshal(w.Body.Bytes(), &valor); err != nil {
		t.Fatalf("respuesta no es JSON: %v: %s", err, w.Body)
	}
	return valor
}

func TestSaludDisponible(t *testing.T) {
	w := pedir(nuevoServidor(nil), "GET", "/health", "")
	salud := decodificar[respuestaSalud](t, w)
	if w.Code != http.StatusOK || salud.Estado != "disponible" || salud.BaseDatos != "ok" || salud.Version != "prueba" {
		t.Fatalf("HTTP %d %+v", w.Code, salud)
	}
}

func TestSaludSinBaseDatos(t *testing.T) {
	w := pedir(nuevoServidor(dominio.NoDisponible(errors.New("conexión rechazada"))), "GET", "/health", "")
	salud := decodificar[respuestaSalud](t, w)
	if w.Code != http.StatusServiceUnavailable || salud.Estado != "no_disponible" || salud.BaseDatos != "error" {
		t.Fatalf("HTTP %d %+v", w.Code, salud)
	}
}

func TestContratoPublicado(t *testing.T) {
	w := pedir(nuevoServidor(nil), "GET", "/openapi.yaml", "")
	if w.Code != http.StatusOK || w.Body.String() != "openapi: 3.0.3\n" || w.Header().Get("Content-Type") != "application/yaml" {
		t.Fatalf("HTTP %d %q", w.Code, w.Body)
	}
}

func TestRutaDesconocida(t *testing.T) {
	w := pedir(nuevoServidor(nil), "GET", "/no-existe", "")
	cuerpo := decodificar[cuerpoError](t, w)
	if w.Code != http.StatusNotFound || cuerpo.Codigo != "NO_ENCONTRADO" || cuerpo.RequestID != w.Header().Get("X-Request-Id") {
		t.Fatalf("HTTP %d %+v", w.Code, cuerpo)
	}
}

func TestRequestID(t *testing.T) {
	servidor := nuevoServidor(nil)
	generado := pedir(servidor, "GET", "/health", "").Header().Get("X-Request-Id")
	if !formatoUUID.MatchString(generado) || generado[14] != '4' {
		t.Fatalf("se esperaba un UUID v4, se obtuvo %q", generado)
	}

	propio := "2e7a4c9b-6d1f-4b8e-a3c5-7f0d2b9e4a1c"
	if id := pedir(servidor, "GET", "/health", "", "X-Request-Id", propio).Header().Get("X-Request-Id"); id != propio {
		t.Fatalf("debe reutilizar el X-Request-Id del consumidor, devolvió %q", id)
	}
	if id := pedir(servidor, "GET", "/health", "", "X-Request-Id", "inyección\n").Header().Get("X-Request-Id"); id == "inyección\n" || !formatoUUID.MatchString(id) {
		t.Fatalf("un X-Request-Id inválido debe reemplazarse, devolvió %q", id)
	}
}

func TestControlDeAdmision(t *testing.T) {
	ocupado, liberar := make(chan struct{}), make(chan struct{})
	lento := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lento" {
			ocupado <- struct{}{}
			<-liberar
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := asignarRequestID(controlarAdmision(lento, 1, map[string]bool{"/health": true}))

	var espera sync.WaitGroup
	espera.Add(1)
	go func() {
		defer espera.Done()
		pedir(h, "GET", "/lento", "")
	}()
	<-ocupado // la única plaza está tomada

	w := pedir(h, "GET", "/otra", "")
	cuerpo := decodificar[cuerpoError](t, w)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" || cuerpo.Codigo != "SOBRECARGA" || cuerpo.RequestID == "" {
		t.Fatalf("sin cupo: HTTP %d %+v", w.Code, cuerpo)
	}
	if w := pedir(h, "GET", "/health", ""); w.Code != http.StatusNoContent {
		t.Fatalf("/health no pasa por la admisión: HTTP %d", w.Code)
	}

	close(liberar)
	espera.Wait()
	if w := pedir(h, "GET", "/otra", ""); w.Code != http.StatusNoContent {
		t.Fatalf("con cupo libre: HTTP %d", w.Code)
	}
}

func TestPanicRespondeErrorInterno(t *testing.T) {
	falla := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("falla inesperada") })
	w := pedir(asignarRequestID(recuperarPanico(falla, logSilencioso)), "GET", "/", "")
	cuerpo := decodificar[cuerpoError](t, w)
	if w.Code != http.StatusInternalServerError || cuerpo.Codigo != "ERROR_INTERNO" || strings.Contains(cuerpo.Mensaje, "falla") {
		t.Fatalf("HTTP %d %+v", w.Code, cuerpo)
	}
}

func TestResponderError(t *testing.T) {
	for _, caso := range []struct {
		err     error
		status  int
		codigo  string
		mensaje string
	}{
		{dominio.Invalido("Falta el campo tipo", dominio.Detalle{Campo: "tipo", Problema: "obligatorio"}),
			400, "SOLICITUD_INVALIDA", "Falta el campo tipo"},
		{&dominio.Error{Categoria: dominio.ErrSolicitudInvalida, Codigo: "LOTE_FUERA_DE_RANGO", Mensaje: "De 1 a 100 cambios"},
			400, "LOTE_FUERA_DE_RANGO", "De 1 a 100 cambios"},
		{dominio.NoEncontrado(""), 404, "NO_ENCONTRADO", "El recurso no existe."},
		{dominio.Duplicado("El activo ya existe"), 409, "DUPLICADO", "El activo ya existe"},
		{dominio.NoDisponible(errors.New("dial tcp: refused")), 503, "NO_DISPONIBLE", "No hay conexión con la base de datos."},
		{errors.New("detalle técnico interno"), 500, "ERROR_INTERNO", "Ocurrió un error inesperado."},
		{&dominio.Error{Mensaje: "sin categoría"}, 500, "ERROR_INTERNO", "Ocurrió un error inesperado."},
	} {
		w := httptest.NewRecorder()
		ResponderError(w, httptest.NewRequest("GET", "/", nil), caso.err)
		cuerpo := decodificar[cuerpoError](t, w)
		if w.Code != caso.status || cuerpo.Codigo != caso.codigo || cuerpo.Mensaje != caso.mensaje {
			t.Errorf("%v: HTTP %d %+v", caso.err, w.Code, cuerpo)
		}
	}
}

func TestLeerJSON(t *testing.T) {
	type entrada struct {
		Nombre string `json:"nombre"`
	}
	leer := func(cuerpo string) (entrada, error) {
		var destino entrada
		err := LeerJSON(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(cuerpo)), &destino)
		return destino, err
	}

	if valor, err := leer(`{"nombre":"panel"}`); err != nil || valor.Nombre != "panel" {
		t.Fatalf("JSON válido: %+v, %v", valor, err)
	}
	for _, cuerpo := range []string{
		`{"nombre":"panel","extra":1}`, // campo desconocido
		`{"nombre":"panel"}{}`,         // contenido sobrante
		`{"nombre":`,                   // JSON incompleto
		`{"nombre":"` + strings.Repeat("a", TamanoMaximoCuerpo) + `"}`,
	} {
		if _, err := leer(cuerpo); !errors.Is(err, dominio.ErrSolicitudInvalida) {
			t.Errorf("cuerpo de %d bytes: se esperaba SolicitudInvalida, se obtuvo %v", len(cuerpo), err)
		}
	}
}
