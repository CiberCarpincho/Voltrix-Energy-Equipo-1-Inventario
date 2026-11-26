package infraestructure

import (
	"context"
	"database/sql"

	"voltrix/inventario/app/business/domain"
)

// Catalogo consulta plantas y activos. Implementa services.RepositorioCatalogo.
type Catalogo struct {
	DB *sql.DB
}

// Consultas parametrizadas. En los listados, un filtro vacío no restringe el resultado.
const (
	sqlContarPlantas = `SELECT count(*) FROM plantas
		WHERE ($1 = '' OR tipo = $1) AND ($2 = '' OR cliente_id = $2)`

	sqlListarPlantas = `SELECT id, nombre, tipo, cliente_id, municipio, capacidad_instalada_kw, creado_en
		FROM plantas
		WHERE ($1 = '' OR tipo = $1) AND ($2 = '' OR cliente_id = $2)
		ORDER BY id LIMIT $3 OFFSET $4`

	sqlBuscarPlanta = `SELECT p.id, p.nombre, p.tipo, p.cliente_id, p.municipio, p.capacidad_instalada_kw, p.creado_en,
			count(a.id),
			count(a.id) FILTER (WHERE a.tipo = 'panel_solar'),
			count(a.id) FILTER (WHERE a.tipo = 'turbina'),
			count(a.id) FILTER (WHERE a.tipo = 'banco_baterias'),
			count(a.id) FILTER (WHERE a.estado = 'operativo'),
			count(a.id) FILTER (WHERE a.estado = 'en_mantenimiento'),
			count(a.id) FILTER (WHERE a.estado = 'fuera_de_servicio')
		FROM plantas p
		LEFT JOIN activos a ON a.planta_id = p.id
		WHERE p.id = $1
		GROUP BY p.id`

	sqlContarActivos = `SELECT count(*) FROM activos
		WHERE ($1 = '' OR planta_id = $1) AND ($2 = '' OR tipo = $2) AND ($3 = '' OR estado = $3)`

	sqlListarActivos = `SELECT id, planta_id, tipo, estado, capacidad_nominal_kw, fecha_instalacion, actualizado_en
		FROM activos
		WHERE ($1 = '' OR planta_id = $1) AND ($2 = '' OR tipo = $2) AND ($3 = '' OR estado = $3)
		ORDER BY id LIMIT $4 OFFSET $5`

	sqlBuscarActivo = `SELECT id, planta_id, tipo, estado, capacidad_nominal_kw, fecha_instalacion, actualizado_en
		FROM activos WHERE id = $1`
)

// ListarPlantas devuelve las plantas ordenadas por identificador.
func (c Catalogo) ListarPlantas(ctx context.Context, filtro domain.FiltroPlantas, pagina domain.Paginacion) (domain.Pagina[domain.Planta], error) {
	resultado := domain.Pagina[domain.Planta]{Datos: []domain.Planta{}, Paginacion: pagina}
	if err := c.DB.QueryRowContext(ctx, sqlContarPlantas, filtro.Tipo, filtro.ClienteID).Scan(&resultado.Total); err != nil {
		return resultado, traducirError(err)
	}

	filas, err := c.DB.QueryContext(ctx, sqlListarPlantas, filtro.Tipo, filtro.ClienteID, pagina.Limite, pagina.Offset)
	if err != nil {
		return resultado, traducirError(err)
	}
	defer filas.Close()
	for filas.Next() {
		planta, err := leerPlanta(filas)
		if err != nil {
			return resultado, traducirError(err)
		}
		resultado.Datos = append(resultado.Datos, planta)
	}
	return resultado, traducirError(filas.Err())
}

// BuscarPlanta devuelve una planta con el conteo de sus activos por tipo y por estado.
func (c Catalogo) BuscarPlanta(ctx context.Context, id string) (domain.DetallePlanta, error) {
	var (
		detalle                                   domain.DetallePlanta
		municipio                                 sql.NullString
		total, panel, turbina, bateria            int
		operativo, mantenimiento, fueraDeServicio int
	)
	if err := c.DB.QueryRowContext(ctx, sqlBuscarPlanta, id).Scan(
		&detalle.ID, &detalle.Nombre, &detalle.Tipo, &detalle.ClienteID, &municipio,
		&detalle.CapacidadInstaladaKw, &detalle.CreadoEn,
		&total, &panel, &turbina, &bateria, &operativo, &mantenimiento, &fueraDeServicio,
	); err != nil {
		return domain.DetallePlanta{}, traducirError(err)
	}
	detalle.Municipio = textoOpcional(municipio)
	detalle.Resumen = domain.ResumenActivos{
		TotalActivos: total,
		PorTipo:      map[string]int{"panel_solar": panel, "turbina": turbina, "banco_baterias": bateria},
		PorEstado: map[string]int{
			"operativo": operativo, "en_mantenimiento": mantenimiento, "fuera_de_servicio": fueraDeServicio,
		},
	}
	return detalle, nil
}

// ListarActivos devuelve los activos ordenados por identificador.
func (c Catalogo) ListarActivos(ctx context.Context, filtro domain.FiltroActivos, pagina domain.Paginacion) (domain.Pagina[domain.Activo], error) {
	resultado := domain.Pagina[domain.Activo]{Datos: []domain.Activo{}, Paginacion: pagina}
	if err := c.DB.QueryRowContext(ctx, sqlContarActivos,
		filtro.PlantaID, filtro.Tipo, filtro.Estado).Scan(&resultado.Total); err != nil {
		return resultado, traducirError(err)
	}

	filas, err := c.DB.QueryContext(ctx, sqlListarActivos,
		filtro.PlantaID, filtro.Tipo, filtro.Estado, pagina.Limite, pagina.Offset)
	if err != nil {
		return resultado, traducirError(err)
	}
	defer filas.Close()
	for filas.Next() {
		activo, err := leerActivo(filas)
		if err != nil {
			return resultado, traducirError(err)
		}
		resultado.Datos = append(resultado.Datos, activo)
	}
	return resultado, traducirError(filas.Err())
}

// BuscarActivo devuelve un activo por su identificador.
func (c Catalogo) BuscarActivo(ctx context.Context, id string) (domain.Activo, error) {
	activo, err := leerActivo(c.DB.QueryRowContext(ctx, sqlBuscarActivo, id))
	return activo, traducirError(err)
}

// escaneable lo cumplen *sql.Row y *sql.Rows.
type escaneable interface {
	Scan(destino ...any) error
}

func leerPlanta(fila escaneable) (domain.Planta, error) {
	var (
		planta    domain.Planta
		municipio sql.NullString
	)
	err := fila.Scan(&planta.ID, &planta.Nombre, &planta.Tipo, &planta.ClienteID, &municipio,
		&planta.CapacidadInstaladaKw, &planta.CreadoEn)
	planta.Municipio = textoOpcional(municipio)
	return planta, err
}

func leerActivo(fila escaneable) (domain.Activo, error) {
	var (
		activo           domain.Activo
		fechaInstalacion sql.NullTime
	)
	err := fila.Scan(&activo.ID, &activo.PlantaID, &activo.Tipo, &activo.Estado,
		&activo.CapacidadNominalKw, &fechaInstalacion, &activo.ActualizadoEn)
	if fechaInstalacion.Valid {
		fecha := fechaInstalacion.Time
		activo.FechaInstalacion = &fecha
	}
	return activo, err
}

func textoOpcional(valor sql.NullString) *string {
	if !valor.Valid {
		return nil
	}
	return &valor.String
}
