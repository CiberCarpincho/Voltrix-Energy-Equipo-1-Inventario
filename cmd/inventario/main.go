// Comando inventario arranca la API. Es el único lugar que conoce todas las capas: lee la
// configuración, crea el repositorio, se lo pasa a los servicios y estos a los manejadores HTTP.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voltrix/inventario/internal/config"
	"voltrix/inventario/internal/httpapi"
	"voltrix/inventario/internal/repositorio/postgres"
	"voltrix/inventario/internal/servicio"
)

// version se fija al compilar (-ldflags "-X main.version=<commit>"); la reporta /health.
var version = "dev"

const rutaContrato = "api/openapi.yaml"

func main() {
	if err := ejecutar(); err != nil {
		slog.Error("el servicio terminó con error", "error", err)
		os.Exit(1)
	}
}

func ejecutar() error {
	cfg, err := config.Cargar(os.Getenv)
	if err != nil {
		return fmt.Errorf("configuración: %w", err)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.NivelLog})))

	// SIGTERM llega cuando Docker detiene o reemplaza el contenedor.
	ctx, detener := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer detener()

	// Capa de acceso a datos
	db, err := postgres.Abrir(cfg.DBMaxConexiones)
	if err != nil {
		return err
	}
	defer db.Close()

	ctxMigracion, cancelar := context.WithTimeout(ctx, 30*time.Second)
	aplicadas, err := postgres.Migrar(ctxMigracion, db)
	cancelar()
	if err != nil {
		return fmt.Errorf("migraciones: %w", err)
	}
	slog.Info("migraciones al día", "aplicadas", aplicadas)

	// Capa de lógica de negocio: recibe los repositorios a través de sus puertos.
	salud := servicio.NuevaSalud(postgres.Salud{DB: db})

	// Capa de presentación
	contrato, err := os.ReadFile(rutaContrato)
	if err != nil {
		return fmt.Errorf("leer el contrato: %w", err)
	}
	servidor := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Puerto),
		Handler: httpapi.Nuevo(httpapi.Dependencias{
			Salud:           salud,
			Contrato:        contrato,
			Version:         version,
			MaxConcurrentes: cfg.AdmisionMaxConcurrentes,
			Log:             slog.Default(),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errServidor := make(chan error, 1)
	go func() {
		slog.Info("API escuchando", "puerto", cfg.Puerto, "version", version,
			"workers", cfg.Workers, "dbMaxConexiones", cfg.DBMaxConexiones)
		errServidor <- servidor.ListenAndServe()
	}()

	select {
	case err := <-errServidor:
		return err
	case <-ctx.Done():
	}

	// Apagado ordenado: deja terminar las peticiones en curso.
	slog.Info("apagando el servicio")
	ctxApagado, cancelarApagado := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelarApagado()
	if err := servidor.Shutdown(ctxApagado); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
