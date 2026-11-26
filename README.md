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
| `deploy/compose/` | Despliegue de API y PostgreSQL en una red privada con volumen, límites de memoria y política de reinicio. `compose.yaml` es el de la EC2; `compose.override.yaml` construye la imagen en local y en el CI. |
| `.github/workflows/` | CI (linter y Semgrep en PR a `develop`), CD (imagen en GHCR y despliegue en EC2 al fusionar en `main`). |
| `deploy/kubernetes/` | Manifiestos para Deployment, Service, ConfigMap, Secret y persistencia en Floci. Dos réplicas para el Avance 2. |
| `benchmarks/` | Medición local del núcleo con la misma semilla, comparación de resultados y tiempos por número de workers. |

## Avance 1 · 26 de noviembre

HU-03 carga 80 plantas y 1.270 activos deterministas en un PostgreSQL vacío al iniciar. La carga solo se aplica si el catálogo está vacío. Compose conserva la base en el volumen `pgdata`.

```powershell
cd deploy/compose
$env:POSTGRES_PASSWORD = "clave-local-de-prueba"
docker compose up -d --build --wait   # compose.yaml + compose.override.yaml
curl.exe http://localhost:8080/activos/PL-001-PS-07
curl.exe "http://localhost:8080/plantas?limit=1"
curl.exe "http://localhost:8080/activos?limit=1"
curl.exe http://localhost:8080/openapi.yaml
docker compose restart
```

Los GET de plantas y activos consultan PostgreSQL. Los POST de este avance aceptan la petición de ejemplo del OpenAPI y devuelven una respuesta de demostración marcada con `X-Inventario-Demo: true`. Se pueden repetir sin modificar la base. Las operaciones funcionales corresponden a HU-06 a HU-10.

Ejecuta `go test ./...` para verificar el contrato y las respuestas de demostración. Para incluir la prueba de integración, configura `INVENTARIO_TEST_DSN` con una conexión PostgreSQL cuyo usuario pueda crear esquemas. La prueba usa su propio esquema para comprobar la semilla, las consultas y la persistencia tras reconectar e iniciar de nuevo.

Con la base detenida (`docker compose stop db`), `/health` responde HTTP 503; al iniciarla de nuevo (`docker compose start db`), la misma API vuelve a responder 200.

## CI/CD y despliegue

**Integración continua** (`.github/workflows/ci.yml`), en cada Pull Request hacia `develop`:

1. Linter: `golangci-lint` con la configuración de `.golangci.yml` (errcheck, govet, staticcheck, ineffassign, unused y formato `gofmt`). Localmente: `golangci-lint run`.
2. Seguridad: Semgrep con las reglas OWASP Top 10, Go, secretos, Dockerfile, Compose y GitHub Actions.

**Despliegue continuo** (`.github/workflows/cd.yml`), en cada push a `main` (al fusionar `develop`) o a mano desde *Actions → CD → Run workflow*: se publica la imagen en GHCR (`ghcr.io/cibercarpincho/voltrix-inventario-api:<commit>` y `:latest`), se copia `deploy/compose/compose.yaml` a la EC2 por SSH, se reinician los contenedores y se verifica `/health`. Si la versión nueva no responde, el pipeline vuelve a la imagen que estaba corriendo y el job falla. Los cambios de esquema de base de datos deben admitir la versión anterior de la API.

**Configuración única en GitHub** (la hace el propietario del repositorio):

1. *Settings → Branches*: regla para `develop` con "Require a pull request before merging" y los checks obligatorios **Linter (golangci-lint)**, **Análisis de seguridad (Semgrep)**; regla para `main` con "Require a pull request before merging" (solo se fusiona `develop`).
2. *Settings → Secrets and variables → Actions*: `EC2_HOST`, `EC2_USUARIO` (`ubuntu`) y `EC2_LLAVE_SSH`; opcionalmente la variable `EC2_DIRECTORIO` (por defecto `voltrix-inventario`, en el home del usuario).

**EC2 (una sola vez)**: Docker con el plugin Compose, el usuario SSH en el grupo `docker`, y la carpeta del despliegue con su `.env` (ver `deploy/compose/.env.example`). Grupo de seguridad: 22 (SSH del pipeline, protegido por la llave) y 8080 (API).

```bash
# En la EC2, para comprobar el estado
cd ~/voltrix-inventario
docker compose ps
curl -fsS http://localhost:8080/health
```

**Volver a una versión anterior** (en la EC2):
`API_IMAGEN=ghcr.io/cibercarpincho/voltrix-inventario-api API_TAG=<commit anterior> docker compose up -d api`.

**Probar el despliegue desde fuera de la EC2** (HU-04, PowerShell; reemplazar la IP):

```powershell
$base = 'http://IP-PUBLICA:8080'
$salud = Invoke-RestMethod "$base/health" -TimeoutSec 10
if ($salud.estado -ne 'disponible' -or $salud.baseDatos -ne 'ok') { throw 'Falló salud' }
$activo = Invoke-RestMethod "$base/activos/PL-001-PS-07" -TimeoutSec 10
if ($activo.id -ne 'PL-001-PS-07' -or $activo.plantaId -ne 'PL-001') { throw 'Activo inesperado' }
Test-NetConnection ([uri]$base).Host -Port 5432 -InformationLevel Quiet   # debe dar False
```

Se espera salud `disponible`, el panel solar de PL-001 y `False` en 5432 (PostgreSQL no se publica fuera de la EC2).
