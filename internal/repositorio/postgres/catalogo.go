package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type Catalogo struct {
	DB *sql.DB
}

const camposPlanta = `id, nombre, tipo, cliente_id AS "clienteId", municipio,
	capacidad_instalada_kw AS "capacidadInstaladaKw", replace(to_char(creado_en AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), '.000000Z', 'Z') AS "creadoEn"`

const camposActivo = `id, planta_id AS "plantaId", tipo, estado,
	capacidad_nominal_kw AS "capacidadNominalKw", fecha_instalacion AS "fechaInstalacion",
	replace(to_char(actualizado_en AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), '.000000Z', 'Z') AS "actualizadoEn"`

const planta = `SELECT ` + camposPlanta + ` FROM plantas`
const activo = `SELECT ` + camposActivo + ` FROM activos`

func (c Catalogo) Plantas(ctx context.Context, tipo, cliente string, limite, offset int) (json.RawMessage, error) {
	return c.pagina(ctx, planta+` WHERE ($1 = '' OR tipo = $1) AND ($2 = '' OR cliente_id = $2)`,
		limite, offset, tipo, cliente)
}

func (c Catalogo) Activos(ctx context.Context, plantaID, tipo, estado string, limite, offset int) (json.RawMessage, error) {
	return c.pagina(ctx, activo+` WHERE ($1 = '' OR planta_id = $1)
		AND ($2 = '' OR tipo = $2) AND ($3 = '' OR estado = $3)`, limite, offset, plantaID, tipo, estado)
}

func (c Catalogo) pagina(ctx context.Context, consulta string, limite, offset int, filtros ...any) (json.RawMessage, error) {
	limiteParam, offsetParam := len(filtros)+1, len(filtros)+2
	return c.json(ctx, fmt.Sprintf(`WITH filtrados AS (%s),
		pagina AS (SELECT * FROM filtrados ORDER BY id LIMIT $%d OFFSET $%d)
		SELECT json_build_object(
			'datos', COALESCE((SELECT json_agg(json_strip_nulls(row_to_json(p)) ORDER BY id) FROM pagina p), '[]'),
			'total', (SELECT count(*) FROM filtrados), 'limit', $%d::integer, 'offset', $%d::bigint
		)`, consulta, limiteParam, offsetParam, limiteParam, offsetParam), append(filtros, limite, offset)...)
}

func (c Catalogo) Planta(ctx context.Context, id string) (json.RawMessage, error) {
	return c.json(ctx, `SELECT json_strip_nulls(row_to_json(detalle)) FROM (
		SELECT `+camposPlanta+`, (
			SELECT json_build_object(
				'totalActivos', count(*),
				'porTipo', json_build_object(
					'panel_solar', count(*) FILTER (WHERE tipo = 'panel_solar'),
					'turbina', count(*) FILTER (WHERE tipo = 'turbina'),
					'banco_baterias', count(*) FILTER (WHERE tipo = 'banco_baterias')
				),
				'porEstado', json_build_object(
					'operativo', count(*) FILTER (WHERE estado = 'operativo'),
					'en_mantenimiento', count(*) FILTER (WHERE estado = 'en_mantenimiento'),
					'fuera_de_servicio', count(*) FILTER (WHERE estado = 'fuera_de_servicio')
				)
			) FROM activos WHERE planta_id = plantas.id
		) AS resumen FROM plantas WHERE id = $1
	) detalle`, id)
}

func (c Catalogo) Activo(ctx context.Context, id string) (json.RawMessage, error) {
	return c.json(ctx, `SELECT json_strip_nulls(row_to_json(a)) FROM (`+activo+` WHERE id = $1) a`, id)
}

func (c Catalogo) json(ctx context.Context, consulta string, args ...any) (json.RawMessage, error) {
	var datos []byte
	err := c.DB.QueryRowContext(ctx, consulta, args...).Scan(&datos)
	return json.RawMessage(datos), err
}
