package domain

import (
	"errors"
	"testing"
)

func TestCategoriasDeError(t *testing.T) {
	causa := errors.New("conexión rechazada")
	for _, caso := range []struct {
		err       error
		categoria error
		texto     string
	}{
		{Invalido("El campo tipo es obligatorio"), ErrSolicitudInvalida, "El campo tipo es obligatorio"},
		{NoEncontrado("Activo desconocido"), ErrNoEncontrado, "Activo desconocido"},
		{Duplicado("El activo ya existe"), ErrDuplicado, "El activo ya existe"},
		{NoDisponible(causa), ErrNoDisponible, "base de datos no disponible: conexión rechazada"},
	} {
		if !errors.Is(caso.err, caso.categoria) {
			t.Errorf("%v no pertenece a la categoría %v", caso.err, caso.categoria)
		}
		if caso.err.Error() != caso.texto {
			t.Errorf("texto: %q, se esperaba %q", caso.err.Error(), caso.texto)
		}
	}
	if !errors.Is(NoDisponible(causa), causa) {
		t.Error("errors.Is debe encontrar la causa original")
	}
}

func TestErrorSinCategoriaNiCausa(t *testing.T) {
	err := &Error{Mensaje: "solo mensaje"}
	if err.Error() != "solo mensaje" || len(err.Unwrap()) != 0 {
		t.Fatalf("error mínimo: %q %v", err.Error(), err.Unwrap())
	}
}
