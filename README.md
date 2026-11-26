# Voltrix Energy · Equipo 1 · Inventario de Activos

Estructura de trabajo basada en las vistas lógica y de despliegue del DAS. Los nombres de directorio son una propuesta de organización para el equipo.

| Carpeta | Responsabilidad |
|---|---|
| `cmd/inventario/` | Punto de entrada de la API en Go. Conecta la configuración, el repositorio, los servicios y el servidor HTTP. |
| `api/` | Contrato OpenAPI 1.0.0 de Inventario. La API deberá servir este archivo en GET /openapi.yaml. |
| `internal/httpapi/` | Rutas y manejadores net/http, decodificación JSON, X-Request-Id, logs y control de admisión. |
| `internal/dominio/` | Modelos de planta y activo, estados operativos y transiciones válidas. |
| `internal/servicio/` | Casos de uso del catálogo, consultas y coordinación de las actualizaciones de estado. |
| `internal/lotes/` | Núcleo de procesamiento secuencial y paralelo por activo, invocable desde la API y desde el benchmark. |
| `internal/repositorio/postgres/` | Consultas parametrizadas y transacciones con bloqueo de fila para conservar la consistencia entre réplicas. |
| `internal/config/` | Lectura de configuración por entorno, límites de admisión, conexiones y número de workers. |
| `db/migraciones/` | Esquema PostgreSQL de plantas y activos, claves e índices. |
| `db/semillas/` | Catálogo determinista común a local, EC2 y Floci. El DAS prevé 80 plantas y 1.270 activos. |
| `infra/ec2/` | IaC y arranque automatizado de EC2 con swap, Docker y Compose. |
| `infra/floci/` | IaC para crear el clúster EKS emulado e IAM de mínimo privilegio. |
| `deploy/compose/` | Despliegue de API y PostgreSQL en una red privada con volumen, límites de memoria y política de reinicio. |
| `deploy/kubernetes/` | Manifiestos para Deployment, Service, ConfigMap, Secret y persistencia en Floci. Dos réplicas para el Avance 2. |
| `benchmarks/` | Medición local del núcleo con la misma semilla, comparación de resultados y tiempos por número de workers. |

## Avance 1 · 26 de noviembre

El esqueleto de carpetas prepara la implementación. Para sustentar el avance se requiere mostrar la API con datos de ejemplo conforme al contrato en EC2 y el clúster Floci EKS creado.

La [guía de despliegue de HU-04](infra/ec2/HU-04.md) reúne los pasos para ejecutar Inventario con PostgreSQL, crear la instancia con CloudFormation y comprobar el acceso desde fuera de EC2.
