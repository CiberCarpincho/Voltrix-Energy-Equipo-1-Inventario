# Voltrix Energy · Equipo 1 · Inventario de Activos

Servicio raíz de Voltrix Energy: mantiene el catálogo de plantas y de activos (paneles solares, turbinas y bancos de baterías) con su estado operativo. Lo consumen Telemetría (equipo 2), Reportes (4), Pronóstico (5) y Mantenimiento (6). El contrato HTTP está en [api/openapi.yaml](api/openapi.yaml).

Este repositorio es el **arquetipo** del servicio: la arquitectura del DAS ya está montada y funcionando, pero no tiene ninguna historia de usuario implementada. Cada HU se agrega siguiendo la [sección 4](#4-cómo-implementar-una-historia-de-usuario).

---

## 1. Arquitectura

Tres capas (DAS, secciones 1.5 y 2). **Cada capa solo habla con la de abajo y solo el repositorio conoce SQL.**

```
            Equipos 2, 4, 5 y 6
                    │ HTTP :8080
┌───────────────────▼────────────────────┐
│ Presentación     internal/httpapi      │  JSON, límites de tamaño y admisión, errores → HTTP
├────────────────────────────────────────┤
│ Lógica de negocio internal/servicio    │  casos de uso y validaciones
│                   internal/lotes       │  pool de workers (procesamiento paralelo)
├────────────────────────────────────────┤
│ Acceso a datos   internal/repositorio/ │  consultas parametrizadas y migraciones
│                  postgres              │
└───────────────────┬────────────────────┘
                    ▼
               PostgreSQL 16

internal/dominio  → modelos y errores de negocio, los usan todas las capas
internal/config   → configuración por variables de entorno
cmd/inventario    → único lugar que conoce todas las capas y las conecta
```

**Puertos.** La capa de negocio no importa el repositorio: declara lo que necesita como interfaces en [internal/servicio/puertos.go](internal/servicio/puertos.go) y `internal/repositorio/postgres` las implementa. `cmd/inventario/main.go` conecta una con otra. Así, en las pruebas el servicio recibe un doble en lugar de PostgreSQL.

**Las reglas se verifican en el CI.** `depguard` (en [.golangci.yml](.golangci.yml)) hace fallar el linter si, por ejemplo, un manejador HTTP importa el repositorio o `database/sql`, o si el servicio importa `net/http`.

## 2. Estructura

| Carpeta | Capa | Contenido |
|---|---|---|
| `cmd/inventario/` | Arranque | Lee la configuración, conecta las capas, aplica migraciones, levanta el servidor y lo apaga en orden ante SIGTERM. |
| `internal/httpapi/` | Presentación | `servidor.go` (rutas y dependencias), `middleware.go`, `respuestas.go` (JSON y errores), `salud.go`. |
| `internal/servicio/` | Negocio | `puertos.go` (interfaces hacia el repositorio) y los casos de uso. |
| `internal/lotes/` | Negocio | Pool de workers del DAS. Vacío: lo implementa la HU de actualización por lotes (ver `doc.go`). |
| `internal/repositorio/postgres/` | Acceso a datos | Pool de conexiones, migraciones (`migraciones/*.sql`), traducción de errores del driver y repositorios. |
| `internal/dominio/` | Compartida | Errores de negocio. Aquí van los modelos (planta, activo, estados). |
| `internal/config/` | Compartida | Variables de entorno con valores por defecto y validación. |
| `api/` | Contrato | OpenAPI 1.0.0, servido en `GET /openapi.yaml`. |
| `deploy/compose/` | Despliegue | `compose.yaml` (EC2) y `compose.override.yaml` (local, construye la imagen). |
| `deploy/kubernetes/` | Despliegue | Manifiestos para Floci EKS (Avance 2). Vacío. |
| `benchmarks/` | Medición | Benchmark secuencial vs. paralelo del DAS. Vacío. |
| `.github/workflows/` | CI/CD | CI (linter y Semgrep) y CD (GHCR y EC2). |

## 3. Qué ya está listo

| Táctica del DAS | Dónde |
|---|---|
| Patrón de capas | `internal/httpapi` → `internal/servicio` → `internal/repositorio/postgres`, verificado por `depguard` |
| Health check: 200 o 503 si la base no responde en 1 s | `GET /health` atraviesa las tres capas (`salud.go` en cada una) |
| Control de admisión: 429 con `Retry-After` | `controlarAdmision` en `internal/httpapi/middleware.go`; `/health` y `/openapi.yaml` quedan fuera |
| Trazabilidad: duración, ruta, código HTTP, request id y equipo consumidor | `registrarPeticion` (logs JSON) y `X-Request-Id` (UUID, se reutiliza si el consumidor lo envía) |
| Errores del contrato (`codigo`, `mensaje`, `requestId`, `detalles`) | `ResponderError`: categorías de `internal/dominio` → 400, 404, 409, 503; lo demás → 500 `ERROR_INTERNO` sin exponer detalles |
| Límite de tamaño del JSON | `LeerJSON` (1 MiB, rechaza campos desconocidos) |
| Configuración externa | `internal/config` (ver [sección 5](#5-configuración)) |
| Consistencia del esquema | Migraciones embebidas, en orden y con bloqueo para varias réplicas |
| Reinicio automático y límites de memoria | `deploy/compose/compose.yaml` (128 MiB API, 256 MiB PostgreSQL) |
| Apagado ordenado | `cmd/inventario/main.go` (espera las peticiones en curso, máximo 10 s) |

## 4. Cómo implementar una historia de usuario

Ejemplo: `GET /activos/{id}`. Se escribe de abajo hacia arriba y cada paso tiene su prueba.

1. **Esquema**: `internal/repositorio/postgres/migraciones/001_activos.sql` con el `CREATE TABLE`. Al arrancar se aplica sola. Numerar siempre con tres dígitos y nunca editar una migración ya fusionada: los cambios van en una nueva (`002_...`).
2. **Dominio**: `internal/dominio/activo.go` con el tipo `Activo`, sus estados válidos y las reglas que no dependen de nada externo. Sin etiquetas JSON ni SQL.
3. **Puerto**: en `internal/servicio/puertos.go`, la interfaz que necesita el caso de uso:
   ```go
   type RepositorioActivos interface {
       Buscar(ctx context.Context, id string) (dominio.Activo, error)
   }
   ```
4. **Repositorio**: `internal/repositorio/postgres/activos.go` implementa la interfaz con consultas parametrizadas (`$1`, nunca concatenando texto) y pasa cada error por `traducirError`. Así `sql.ErrNoRows` llega arriba como `dominio.ErrNoEncontrado` y una clave repetida como `dominio.ErrDuplicado`.
5. **Caso de uso**: `internal/servicio/activos.go` recibe el puerto en su constructor (`NuevoActivos(repo RepositorioActivos)`), valida y devuelve errores de dominio: `dominio.Invalido(...)`, `dominio.NoEncontrado(...)` o `&dominio.Error{Categoria: ..., Codigo: "PLANTA_DESCONOCIDA", ...}` si el contrato define un código propio.
6. **Manejador**: `internal/httpapi/activos.go` lee la petición (`r.PathValue`, `LeerJSON`), llama al servicio y responde con `EscribirJSON` o `ResponderError`. Los DTO con los nombres JSON del contrato (camelCase) viven aquí y se convierten desde el dominio. Se agrega el caso de uso a `Dependencias` y la ruta en `registrarRutas` (`servidor.go`).
7. **Conexión**: en `cmd/inventario/main.go`, `servicio.NuevoActivos(postgres.Activos{DB: db})` y se pasa en `httpapi.Dependencias`.
8. **Pruebas**:
   - servicio: con un doble del puerto (ver `internal/servicio/salud_test.go`);
   - manejador: con `httptest` y el servicio armado sobre un doble (ver `internal/httpapi/servidor_test.go`);
   - repositorio: de integración contra PostgreSQL con `INVENTARIO_TEST_DSN` (ver `migraciones_integration_test.go`, que usa un esquema propio y lo borra al terminar).

Para la HU de actualización por lotes, el pool de workers va en `internal/lotes` según el diseño de `doc.go` (reparto por `hash(id) mod N` y `config.Workers`). El servicio lo usa y le pasa la función que aplica un cambio con el repositorio (`SELECT ... FOR UPDATE`).

## 5. Configuración

| Variable | Por defecto | Uso |
|---|---|---|
| `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE`, `PGCONNECT_TIMEOUT` | — | Conexión a PostgreSQL (variables estándar de libpq; Compose las fija) |
| `PUERTO` | `8080` | Puerto HTTP |
| `DB_MAX_CONEXIONES` | `4` | Tamaño del pool de conexiones |
| `WORKERS` | `4` | Workers del pool de lotes (no puede superar `DB_MAX_CONEXIONES`) |
| `ADMISION_MAX_CONCURRENTES` | `32` | Peticiones de negocio simultáneas antes de responder 429 |
| `LOG_NIVEL` | `info` | `debug`, `info`, `warn` o `error` |

Si un valor no es válido, el servicio no arranca y el log indica qué variable corregir.

## 6. Desarrollo local

Requisitos: Go 1.26 o superior y Docker con el plugin Compose.

```bash
cd deploy/compose
cp .env.example .env               # poner una POSTGRES_PASSWORD cualquiera
docker compose up -d --build --wait   # compose.yaml + compose.override.yaml
curl http://localhost:8080/health
curl http://localhost:8080/openapi.yaml
```

Pruebas y linter, desde la raíz:

```bash
go test ./...                     # unitarias (sin base de datos)
# Con la base del Compose anterior levantada, también las de integración:
INVENTARIO_TEST_DSN="postgres://inventario:<POSTGRES_PASSWORD>@127.0.0.1:5433/inventario?sslmode=disable" go test ./...
golangci-lint run                 # el mismo linter del CI, con las reglas de capas
```

Con la base detenida (`docker compose stop db`), `/health` responde 503; al iniciarla (`docker compose start db`) vuelve a responder 200 sin reiniciar la API.

## 7. Flujo de trabajo

Cada integrante trabaja en su rama, abre un PR hacia `develop` (lo revisa el CI) y `develop` se fusiona en `main` para desplegar. La plantilla del PR está en [.github/pull_request_template.md](.github/pull_request_template.md).

## 8. CI/CD y despliegue

**Integración continua** (`.github/workflows/ci.yml`), en cada Pull Request hacia `develop`:

1. Linter: `golangci-lint` con la configuración de `.golangci.yml` (errcheck, govet, staticcheck, ineffassign, unused, formato `gofmt` y reglas de capas con `depguard`).
2. Seguridad: Semgrep con las reglas OWASP Top 10, Go, secretos, Dockerfile, Compose y GitHub Actions.

**Despliegue continuo** (`.github/workflows/cd.yml`), en cada push a `main` (al fusionar `develop`) o a mano desde *Actions → CD → Run workflow*: se publica la imagen en GHCR (`ghcr.io/infraestructuras-voltrix/voltrix-inventario-api:<commit>` y `:latest`), se copia `deploy/compose/compose.yaml` a la EC2 por SSH, se reinician los contenedores y se verifica `/health`, que reporta el commit desplegado en `version`. Si la versión nueva no responde, el pipeline vuelve a la imagen que estaba corriendo y el job falla. Los cambios de esquema deben admitir la versión anterior de la API.

**Configuración única en GitHub** (la hace un administrador del repositorio):

1. *Settings → Branches*: regla para `develop` con "Require a pull request before merging" y los checks obligatorios **Linter (golangci-lint)** y **Análisis de seguridad (Semgrep)**; regla para `main` con "Require a pull request before merging" (solo se fusiona `develop`).
2. *Settings → Secrets and variables → Actions*: `EC2_HOST`, `EC2_USUARIO` (`ubuntu`) y `EC2_LLAVE_SSH`; opcionalmente la variable `EC2_DIRECTORIO` (por defecto `voltrix-inventario`, en el home del usuario).

**EC2 (una sola vez)**: Docker con el plugin Compose, el usuario SSH en el grupo `docker`, y la carpeta del despliegue con su `.env` (ver `deploy/compose/.env.example`). Grupo de seguridad: 22 (SSH del pipeline, protegido por la llave) y 8080 (API).

```bash
# En la EC2, para comprobar el estado
cd ~/voltrix-inventario
docker compose ps
curl -fsS http://localhost:8080/health
```

**Volver a una versión anterior** (en la EC2):
`API_IMAGEN=ghcr.io/infraestructuras-voltrix/voltrix-inventario-api API_TAG=<commit anterior> docker compose up -d api`.

**Probar el despliegue desde fuera de la EC2** (PowerShell; reemplazar la IP):

```powershell
$base = 'http://IP-PUBLICA:8080'
$salud = Invoke-RestMethod "$base/health" -TimeoutSec 10
if ($salud.estado -ne 'disponible' -or $salud.baseDatos -ne 'ok') { throw 'Falló salud' }
$salud.version                                                             # commit desplegado
Test-NetConnection ([uri]$base).Host -Port 5432 -InformationLevel Quiet   # debe dar False
```

PostgreSQL no se publica fuera de la EC2, por eso el puerto 5432 debe dar `False`.
