package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"voltrix/inventario/internal/repositorio/postgres"
)

var identificador = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func registrarCatalogo(mux *http.ServeMux, catalogo postgres.Catalogo) {
	mux.HandleFunc("GET /plantas/{plantaId}", detalle("plantaId", catalogo.Planta))
	mux.HandleFunc("GET /activos/{id}", detalle("id", catalogo.Activo))
	mux.HandleFunc("GET /plantas", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tipo, cliente := q.Get("tipo"), q.Get("clienteId")
		if !filtroValido(tipo, "solar", "eolica", "hibrida") || !idValido(cliente) {
			escribirError(w, 400, "SOLICITUD_INVALIDA", "Filtro de plantas inválido")
			return
		}
		listado(w, r, func(ctx context.Context, limite, offset int) (json.RawMessage, error) {
			return catalogo.Plantas(ctx, tipo, cliente, limite, offset)
		})
	})
	mux.HandleFunc("GET /activos", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		planta, tipo, estado := q.Get("plantaId"), q.Get("tipo"), q.Get("estado")
		if !idValido(planta) || !filtroValido(tipo, "panel_solar", "turbina", "banco_baterias") ||
			!filtroValido(estado, "operativo", "en_mantenimiento", "fuera_de_servicio") {
			escribirError(w, 400, "SOLICITUD_INVALIDA", "Filtro de activos inválido")
			return
		}
		listado(w, r, func(ctx context.Context, limite, offset int) (json.RawMessage, error) {
			return catalogo.Activos(ctx, planta, tipo, estado, limite, offset)
		})
	})
}

func idValido(valor string) bool {
	return valor == "" || identificador.MatchString(valor)
}

func filtroValido(valor string, opciones ...string) bool {
	return valor == "" || slices.Contains(opciones, valor)
}

func detalle(parametro string, consultar func(context.Context, string) (json.RawMessage, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		datos, err := consultar(ctx, r.PathValue(parametro))
		responderConsulta(w, datos, err)
	}
}

func listado(w http.ResponseWriter, r *http.Request, consultar func(context.Context, int, int) (json.RawMessage, error)) {
	q := r.URL.Query()
	limite, offset := 50, 0
	var err error
	if q.Has("limit") {
		limite, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limite < 1 || limite > 200 {
			escribirError(w, 400, "SOLICITUD_INVALIDA", "limit debe estar entre 1 y 200")
			return
		}
	}
	if q.Has("offset") {
		offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil || offset < 0 {
			escribirError(w, 400, "SOLICITUD_INVALIDA", "offset debe ser un entero no negativo")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	datos, err := consultar(ctx, limite, offset)
	responderConsulta(w, datos, err)
}

func responderConsulta(w http.ResponseWriter, datos json.RawMessage, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		escribirError(w, 404, "NO_ENCONTRADO", "Recurso desconocido")
	case err != nil:
		escribirError(w, 503, "NO_DISPONIBLE", "No se pudo consultar el catálogo")
	default:
		writeJSON(w, 200, datos)
	}
}

func escribirError(w http.ResponseWriter, status int, codigo, mensaje string) {
	writeJSON(w, status, map[string]string{
		"codigo": codigo, "mensaje": mensaje, "requestId": w.Header().Get("X-Request-Id"),
	})
}
