package config

import (
	"log/slog"
	"strings"
	"testing"
)

func entorno(valores map[string]string) func(string) string {
	return func(nombre string) string { return valores[nombre] }
}

func TestCargarValoresPorDefecto(t *testing.T) {
	c, err := Cargar(entorno(nil))
	if err != nil {
		t.Fatal(err)
	}
	esperado := Config{Puerto: 8080, DBMaxConexiones: 4, Workers: 4, AdmisionMaxConcurrentes: 32, NivelLog: slog.LevelInfo}
	if c != esperado {
		t.Fatalf("configuración por defecto: %+v", c)
	}
}

func TestCargarDesdeVariables(t *testing.T) {
	c, err := Cargar(entorno(map[string]string{
		"PUERTO": "9000", "DB_MAX_CONEXIONES": "8", "WORKERS": "6",
		"ADMISION_MAX_CONCURRENTES": "10", "LOG_NIVEL": "debug",
	}))
	if err != nil {
		t.Fatal(err)
	}
	esperado := Config{Puerto: 9000, DBMaxConexiones: 8, Workers: 6, AdmisionMaxConcurrentes: 10, NivelLog: slog.LevelDebug}
	if c != esperado {
		t.Fatalf("configuración leída: %+v", c)
	}
}

func TestCargarRechazaValoresInvalidos(t *testing.T) {
	for _, caso := range []struct {
		variables map[string]string
		mensaje   string
	}{
		{map[string]string{"PUERTO": "abc"}, "PUERTO"},
		{map[string]string{"PUERTO": "70000"}, "PUERTO"},
		{map[string]string{"WORKERS": "0"}, "WORKERS"},
		{map[string]string{"LOG_NIVEL": "ruidoso"}, "LOG_NIVEL"},
		{map[string]string{"WORKERS": "5", "DB_MAX_CONEXIONES": "4"}, "no puede superar"},
	} {
		_, err := Cargar(entorno(caso.variables))
		if err == nil || !strings.Contains(err.Error(), caso.mensaje) {
			t.Errorf("%v: se esperaba un error con %q, se obtuvo %v", caso.variables, caso.mensaje, err)
		}
	}
}
