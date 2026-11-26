// Package config lee la configuración del servicio desde variables de entorno (DAS 2.1:
// "las direcciones de los servicios, el número de workers y el tamaño del pool de
// conexiones se leen de variables de entorno, nunca del código").
//
// La conexión a PostgreSQL usa las variables estándar de libpq (PGHOST, PGPORT,
// PGDATABASE, PGUSER, PGPASSWORD, PGSSLMODE, PGCONNECT_TIMEOUT), que lee el driver.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// Config reúne los parámetros del servicio. Los valores por defecto siguen el
// presupuesto de memoria de la EC2 de 1 GiB (DAS, vista física).
type Config struct {
	// Puerto HTTP de la API (PUERTO).
	Puerto int
	// Conexiones máximas del pool hacia PostgreSQL (DB_MAX_CONEXIONES).
	DBMaxConexiones int
	// Workers del pool que procesa los lotes de cambios de estado (WORKERS).
	// No puede superar DBMaxConexiones: un worker sin conexión solo espera.
	Workers int
	// Peticiones de negocio atendidas a la vez; las demás reciben 429
	// (ADMISION_MAX_CONCURRENTES).
	AdmisionMaxConcurrentes int
	// Nivel de los logs: debug, info, warn o error (LOG_NIVEL).
	NivelLog slog.Level
}

// Cargar lee la configuración con la función dada (os.Getenv en producción) y la valida.
func Cargar(getenv func(string) string) (Config, error) {
	c := Config{}
	var errs []error
	c.Puerto = entero(getenv, "PUERTO", 8080, &errs)
	c.DBMaxConexiones = entero(getenv, "DB_MAX_CONEXIONES", 4, &errs)
	c.Workers = entero(getenv, "WORKERS", 4, &errs)
	c.AdmisionMaxConcurrentes = entero(getenv, "ADMISION_MAX_CONCURRENTES", 32, &errs)

	if nivel := strings.TrimSpace(getenv("LOG_NIVEL")); nivel != "" {
		if err := c.NivelLog.UnmarshalText([]byte(nivel)); err != nil {
			errs = append(errs, fmt.Errorf("LOG_NIVEL: %q no es debug, info, warn ni error", nivel))
		}
	}
	if c.Puerto > 65535 {
		errs = append(errs, fmt.Errorf("PUERTO: %d no es un puerto válido", c.Puerto))
	}
	if c.Workers > c.DBMaxConexiones {
		errs = append(errs, fmt.Errorf("WORKERS (%d) no puede superar DB_MAX_CONEXIONES (%d)",
			c.Workers, c.DBMaxConexiones))
	}
	return c, errors.Join(errs...)
}

// entero lee un entero positivo; si la variable no existe devuelve el valor por defecto.
func entero(getenv func(string) string, nombre string, porDefecto int, errs *[]error) int {
	valor := strings.TrimSpace(getenv(nombre))
	if valor == "" {
		return porDefecto
	}
	n, err := strconv.Atoi(valor)
	if err != nil || n < 1 {
		*errs = append(*errs, fmt.Errorf("%s: %q debe ser un entero mayor que 0", nombre, valor))
		return porDefecto
	}
	return n
}
