-- Catálogo de ejemplo determinista: 80 plantas y 1.270 activos (datos del primer avance).
-- ON CONFLICT DO NOTHING: no falla si la base ya tenía el catálogo de una versión anterior.
INSERT INTO plantas (id, nombre, tipo, cliente_id, municipio, capacidad_instalada_kw, creado_en)
VALUES
    ('PL-001', 'Microplanta Solar Jamundí', 'solar', 'CLI-COOP-01', 'Jamundí', 120.5, '2026-11-19T13:00:00Z'),
    ('PL-002', 'Microplanta Eólica La Cumbre', 'eolica', 'CLI-COOP-01', 'La Cumbre', 200, '2026-11-19T13:00:00Z'),
    ('PL-003', 'Microplanta Híbrida Dagua', 'hibrida', 'CLI-IND-04', 'Dagua', 180, '2026-11-19T13:00:00Z')
ON CONFLICT (id) DO NOTHING;

INSERT INTO plantas (id, nombre, tipo, cliente_id, capacidad_instalada_kw, creado_en)
SELECT
    'PL-' || lpad(numero::text, 3, '0'),
    'Microplanta ' || numero,
    CASE WHEN numero <= 42 THEN 'solar' WHEN numero <= 66 THEN 'eolica' ELSE 'hibrida' END,
    'CLI-DEMO-' || lpad(((numero - 4) % 10 + 1)::text, 2, '0'),
    CASE WHEN numero <= 42 THEN 11 WHEN numero <= 66 THEN 200 ELSE 105.5 END,
    '2026-11-19T13:00:00Z'
FROM generate_series(4, 80) AS serie(numero)
ON CONFLICT (id) DO NOTHING;

INSERT INTO activos (id, planta_id, tipo, estado, capacidad_nominal_kw, fecha_instalacion, actualizado_en)
SELECT
    p.id || '-' || modelo.codigo || '-' || lpad(numero::text, 2, '0'),
    p.id,
    modelo.tipo,
    CASE WHEN (p.id, modelo.codigo, numero) IN (('PL-002', 'TB', 1), ('PL-003', 'PS', 1))
        THEN 'en_mantenimiento' ELSE 'operativo' END,
    modelo.capacidad,
    CASE WHEN p.id = 'PL-002' THEN DATE '2024-08-02' ELSE DATE '2025-03-14' END,
    CASE WHEN (p.id, modelo.codigo, numero) = ('PL-002', 'TB', 1)
        THEN TIMESTAMPTZ '2026-11-19T14:00:00Z' ELSE p.creado_en END
FROM plantas p
JOIN (VALUES
    ('solar', 'PS', 'panel_solar', 20, 0.55),
    ('solar', 'BB', 'banco_baterias', 2, 30),
    ('eolica', 'TB', 'turbina', 4, 50),
    ('eolica', 'BB', 'banco_baterias', 2, 30),
    ('hibrida', 'PS', 'panel_solar', 10, 0.55),
    ('hibrida', 'TB', 'turbina', 2, 50),
    ('hibrida', 'BB', 'banco_baterias', 4, 30)
) AS modelo(planta_tipo, codigo, tipo, cantidad, capacidad) ON modelo.planta_tipo = p.tipo
CROSS JOIN LATERAL generate_series(1, modelo.cantidad) AS serie(numero)
ON CONFLICT (id) DO NOTHING;
