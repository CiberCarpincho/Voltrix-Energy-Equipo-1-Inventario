-- Catálogo de plantas y activos (esquema del primer avance).
CREATE TABLE IF NOT EXISTS plantas (
    id text PRIMARY KEY,
    nombre text NOT NULL,
    tipo text NOT NULL CHECK (tipo IN ('solar', 'eolica', 'hibrida')),
    cliente_id text NOT NULL,
    municipio text,
    capacidad_instalada_kw numeric NOT NULL CHECK (capacidad_instalada_kw >= 0),
    creado_en timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS activos (
    id text PRIMARY KEY,
    planta_id text NOT NULL REFERENCES plantas(id),
    tipo text NOT NULL CHECK (tipo IN ('panel_solar', 'turbina', 'banco_baterias')),
    estado text NOT NULL CHECK (estado IN ('operativo', 'en_mantenimiento', 'fuera_de_servicio')),
    capacidad_nominal_kw numeric NOT NULL CHECK (capacidad_nominal_kw >= 0),
    fecha_instalacion date,
    actualizado_en timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS activos_planta_idx ON activos(planta_id);
CREATE INDEX IF NOT EXISTS activos_estado_idx ON activos(estado);
