// Package domain contiene los modelos y reglas del negocio que comparten todas las capas
// (plantas, activos, estados operativos y sus transiciones). No depende de HTTP ni de SQL.
//
// Cada historia de usuario agrega aquí sus tipos, por ejemplo activo.go con el tipo Activo
// y los estados válidos.
package domain

import "errors"

// Categorías de error. La capa de presentación traduce cada una a un código HTTP
// (400, 404, 409 y 503); cualquier otro error se responde como 500 ERROR_INTERNO.
var (
	ErrSolicitudInvalida = errors.New("solicitud inválida")
	ErrNoEncontrado      = errors.New("recurso no encontrado")
	ErrDuplicado         = errors.New("recurso duplicado")
	ErrNoDisponible      = errors.New("base de datos no disponible")
)

// Detalle describe el problema de un campo concreto de la solicitud.
type Detalle struct {
	Campo    string
	Problema string
}

// Error es un error de negocio con la información que ve el consumidor.
type Error struct {
	// Categoria es una de las variables Err* de este paquete.
	Categoria error
	// Codigo es el código del contrato OpenAPI (por ejemplo PLANTA_DESCONOCIDA).
	// Vacío: se usa el código por defecto de la categoría.
	Codigo string
	// Mensaje para el consumidor. Vacío: se usa el mensaje por defecto de la categoría.
	Mensaje  string
	Detalles []Detalle
	// Causa es el error técnico original (por ejemplo, del driver); solo va a los logs.
	Causa error
}

func (e *Error) Error() string {
	mensaje := e.Mensaje
	if mensaje == "" && e.Categoria != nil {
		mensaje = e.Categoria.Error()
	}
	if e.Causa != nil {
		return mensaje + ": " + e.Causa.Error()
	}
	return mensaje
}

// Unwrap permite usar errors.Is con la categoría y con la causa.
func (e *Error) Unwrap() []error {
	var errs []error
	for _, err := range []error{e.Categoria, e.Causa} {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Invalido indica que la solicitud no cumple las reglas del negocio o del contrato (400).
func Invalido(mensaje string, detalles ...Detalle) *Error {
	return &Error{Categoria: ErrSolicitudInvalida, Mensaje: mensaje, Detalles: detalles}
}

// NoEncontrado indica que el recurso pedido no existe (404).
func NoEncontrado(mensaje string) *Error {
	return &Error{Categoria: ErrNoEncontrado, Mensaje: mensaje}
}

// Duplicado indica que el recurso ya existe (409).
func Duplicado(mensaje string) *Error {
	return &Error{Categoria: ErrDuplicado, Mensaje: mensaje}
}

// NoDisponible indica que la base de datos no responde (503).
func NoDisponible(causa error) *Error {
	return &Error{Categoria: ErrNoDisponible, Causa: causa}
}
