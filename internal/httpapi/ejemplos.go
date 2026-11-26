package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
)

type ejemplo struct {
	Ref   string          `json:"$ref"`
	Value json.RawMessage `json:"value"`
}

type contenido struct {
	Example  json.RawMessage
	Examples map[string]ejemplo
}

// registrarEjemplos habilita los cuerpos de demostración publicados en el
// contrato del primer avance. Las operaciones funcionales se incorporan en HU-06 a HU-10.
func registrarEjemplos(mux *http.ServeMux, contrato []byte) error {
	var spec struct {
		Paths map[string]struct {
			Post struct {
				RequestBody struct{ Content map[string]contenido }
				Responses   map[string]struct{ Content map[string]contenido }
			}
		}
		Components struct{ Examples map[string]ejemplo }
	}
	if err := json.Unmarshal(contrato, &spec); err != nil {
		return fmt.Errorf("leer ejemplos del contrato: %w", err)
	}

	for _, caso := range []struct {
		ruta, entrada, salida, location string
		status                          int
	}{
		{"/plantas", "solar", "creada", "/plantas/PL-001", http.StatusCreated},
		{"/activos", "panel", "creado", "/activos/PL-001-PS-07", http.StatusCreated},
		{"/activos/consulta", "", "", "", http.StatusOK},
		{"/activos/actualizaciones-estado", "telemetria", "mixto", "", http.StatusOK},
	} {
		operacion := spec.Paths[caso.ruta].Post
		peticion := operacion.RequestBody.Content["application/json"]
		respuesta := operacion.Responses[fmt.Sprint(caso.status)].Content["application/json"]
		entrada, salida := peticion.Example, respuesta.Example
		if caso.entrada != "" {
			entrada = peticion.Examples[caso.entrada].Value
			e := respuesta.Examples[caso.salida]
			salida = e.Value
			if e.Ref != "" {
				salida = spec.Components.Examples[strings.TrimPrefix(e.Ref, "#/components/examples/")].Value
			}
		}
		var datosEsperados any
		if err := json.Unmarshal(entrada, &datosEsperados); err != nil || !json.Valid(salida) {
			return fmt.Errorf("faltan ejemplos válidos para POST %s", caso.ruta)
		}
		mux.HandleFunc("POST "+caso.ruta, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Inventario-Demo", "true")
			cuerpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
			var datos any
			if err != nil || json.Unmarshal(cuerpo, &datos) != nil || !reflect.DeepEqual(datos, datosEsperados) {
				escribirError(w, 400, "SOLICITUD_INVALIDA", "En el primer avance envíe el cuerpo de ejemplo publicado en el OpenAPI")
				return
			}
			if caso.location != "" {
				w.Header().Set("Location", caso.location)
			}
			writeJSON(w, caso.status, salida)
		})
	}
	return nil
}
