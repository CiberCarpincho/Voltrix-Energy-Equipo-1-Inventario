package services

import (
	"context"
	"errors"
	"testing"
	"time"
)

// verificadorFalso es un doble de prueba del puerto VerificadorBaseDatos.
type verificadorFalso struct {
	err         error
	plazoMaximo time.Duration
}

func (v *verificadorFalso) Verificar(ctx context.Context) error {
	if limite, ok := ctx.Deadline(); ok {
		v.plazoMaximo = time.Until(limite)
	}
	return v.err
}

func TestSaludConBaseDisponible(t *testing.T) {
	verificador := &verificadorFalso{}
	estado := NuevaSalud(verificador).Consultar(context.Background())
	if !estado.BaseDatosDisponible || estado.TiempoActivo < 0 {
		t.Fatalf("estado: %+v", estado)
	}
	if verificador.plazoMaximo <= 0 || verificador.plazoMaximo > EsperaVerificacion {
		t.Fatalf("la verificación debe tener un plazo de hasta %v, tuvo %v", EsperaVerificacion, verificador.plazoMaximo)
	}
}

func TestSaludConBaseCaida(t *testing.T) {
	estado := NuevaSalud(&verificadorFalso{err: errors.New("conexión rechazada")}).Consultar(context.Background())
	if estado.BaseDatosDisponible {
		t.Fatal("la base caída debe reportarse como no disponible")
	}
}
