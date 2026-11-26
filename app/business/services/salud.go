package services

import (
	"context"
	"log/slog"
	"time"
)

// EsperaVerificacion es el tiempo máximo del ping a la base (contrato: 1 s).
const EsperaVerificacion = time.Second

// EstadoSalud es el resultado de verificar el servicio.
type EstadoSalud struct {
	BaseDatosDisponible bool
	TiempoActivo        time.Duration
}

// Salud es el caso de uso de verificación de salud (health check del DAS).
type Salud struct {
	baseDatos VerificadorBaseDatos
	inicio    time.Time
}

// NuevaSalud crea el caso de uso; el tiempo activo se cuenta desde este momento.
func NuevaSalud(baseDatos VerificadorBaseDatos) *Salud {
	return &Salud{baseDatos: baseDatos, inicio: time.Now()}
}

// Consultar verifica la base de datos con un tiempo de espera corto.
func (s *Salud) Consultar(ctx context.Context) EstadoSalud {
	ctx, cancel := context.WithTimeout(ctx, EsperaVerificacion)
	defer cancel()
	err := s.baseDatos.Verificar(ctx)
	if err != nil {
		slog.WarnContext(ctx, "la base de datos no responde", "error", err)
	}
	return EstadoSalud{BaseDatosDisponible: err == nil, TiempoActivo: time.Since(s.inicio)}
}
