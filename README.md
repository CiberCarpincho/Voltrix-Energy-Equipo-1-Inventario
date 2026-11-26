# Voltrix Energy · Equipo 1 · Inventario de Activos

Servicio raíz de Voltrix Energy: mantiene el catálogo de plantas y de activos (paneles solares, turbinas y bancos de baterías) con su estado operativo. Lo consumen Telemetría (equipo 2), Reportes (4), Pronóstico (5) y Mantenimiento (6). El contrato HTTP está en [docs/openapi.yaml](docs/openapi.yaml).

La arquitectura del DAS ya está montada y funcionando. Para el primer avance el servicio publica un **catálogo de ejemplo** según el contrato (80 plantas y 1.270 activos deterministas) y responde los POST con los ejemplos del OpenAPI. Las historias de usuario se agregan siguiendo la [sección 4](#4-cómo-implementar-una-historia-de-usuario).

---

## 1. Arquitectura

Tres capas (DAS, secciones 1.5 y 2). **Cada capa solo habla con la de abajo y solo `app/infrastructure` conoce SQL.**

```
              Equipos 2, 4, 5 y 6
                      │ HTTP :8080
┌─────────────────────▼──────────────────────┐
│ Presentación       app/api                 │  JSON, límites de tamaño y admisión, errores → HTTP
├────────────────────────────────────────────┤
│ Lógica de negocio  app/business/services   │  casos de uso y validaciones
│                    app/business/workers    │  pool de workers (procesamiento paralelo)
├────────────────────────────────────────────┤
│ Acceso a datos     app/infrastructure     │  consultas parametrizadas y migraciones
└─────────────────────┬──────────────────────┘
                      ▼
                 PostgreSQL 16

app/business/domain → modelos y errores de negocio, los usan todas las capas
app/core            → utilidades: configuración por variables de entorno
app/main.go         → único lugar que conoce todas las capas y las conecta
```

**Puertos.** La capa de negocio no importa `app/infrastructure`: declara lo que necesita como interfaces en [app/business/services/puertos.go](app/business/services/puertos.go) y `app/infrastructure` las implementa. `app/main.go` conecta una con otra. Así, en las pruebas el servicio recibe un doble en lugar de PostgreSQL.

**Las reglas se verifican en el CI.** `depguard` (en [.golangci.yml](.golangci.yml)) hace fallar el linter si, por ejemplo, un manejador de `app/api` importa `app/infrastructure` o `database/sql`, o si un servicio importa `net/http`.

## 2. Estructura

| Carpeta | Capa | Contenido |
|---|---|---|
| `app/main.go` | Arranque | Lee la configuración, conecta las capas, aplica migraciones, levanta el servidor y lo apaga en orden ante SIGTERM. |
| `app/api/` | Presentación | `servidor.go` (rutas y dependencias), `middleware.go`, `respuestas.go` (JSON y errores), `salud.go`, `catalogo.go` (GET del catálogo y sus DTO) y `ejemplos.go` (POST de demostración). |
| `app/business/services/` | Negocio | `puertos.go` (interfaces hacia `app/infrastructure`) y los casos de uso (`salud.go`, `catalogo.go`). |
| `app/business/workers/` | Negocio | Pool de workers del DAS. Vacío: lo implementa la HU de actualización por lotes (ver `doc.go`). |
| `app/business/domain/` | Compartida | Errores de negocio y modelos del catálogo (`catalogo.go`: planta, activo, tipos, estados y paginación). |
| `app/infrastructure/` | Acceso a datos | Pool de conexiones, migraciones (`migraciones/*.sql`: esquema y catálogo de ejemplo), traducción de errores del driver y repositorios (`catalogo.go`). |
| `app/core/` | Utilidades | Configuración por variables de entorno con valores por defecto y validación. |
| `docs/` | Contrato | OpenAPI 1.0.0, servido en `GET /openapi.yaml`. |
| `docker-compose.yaml` | Despliegue | API y PostgreSQL; es el que se copia a la EC2. |
| `docker-compose.override.yaml` | Desarrollo | Construye la imagen desde el código y publica PostgreSQL en localhost. |
| `infra/ec2/` | IaC | Plantilla de CloudFormation y `desplegar.sh`, que crea la EC2 y la conecta al CD en un solo comando (ver [sección 8](#8-cicd-y-despliegue)). |
| `.github/workflows/` | CI/CD | CI (linter y Semgrep) y CD (GHCR y EC2). |

## 3. Qué ya está listo

| Táctica del DAS | Dónde |
|---|---|
| Patrón de capas | `app/api` → `app/business/services` → `app/infrastructure`, verificado por `depguard` |
| Health check: 200 o 503 si la base no responde en 1 s | `GET /health` atraviesa las tres capas (`salud.go` en cada una) |
| Control de admisión: 429 con `Retry-After` | `controlarAdmision` en `app/api/middleware.go`; `/health` y `/openapi.yaml` quedan fuera |
| Trazabilidad: duración, ruta, código HTTP, request id y equipo consumidor | `registrarPeticion` (logs JSON) y `X-Request-Id` (UUID, se reutiliza si el consumidor lo envía) |
| Errores del contrato (`codigo`, `mensaje`, `requestId`, `detalles`) | `ResponderError`: categorías de `app/business/domain` → 400, 404, 409, 503; lo demás → 500 `ERROR_INTERNO` sin exponer detalles |
| Límite de tamaño del JSON | `TamanoMaximoCuerpo` = 64 KiB (criterio 7 de HU-INV-03) en `LeerJSON` y en los POST de demostración; `LeerJSON` además rechaza campos desconocidos |
| Configuración externa | `app/core` (ver [sección 5](#5-configuración)) |
| Consistencia del esquema | Migraciones embebidas, en orden y con bloqueo para varias réplicas |
| Reinicio automático y límites de memoria | `docker-compose.yaml` (128 MiB API, 256 MiB PostgreSQL) |
| Apagado ordenado | `app/main.go` (espera las peticiones en curso, máximo 10 s) |

**Endpoints del primer avance**

| Operación | Respuesta |
|---|---|
| `GET /activos/{id}` | Activo del catálogo de ejemplo (404 si no existe) |
| `GET /activos?plantaId=&tipo=&estado=&limit=&offset=` | Página de activos (por defecto 50, máximo 200) |
| `GET /plantas?tipo=&clienteId=&limit=&offset=` | Página de plantas |
| `GET /plantas/{plantaId}` | Planta con el resumen de sus activos por tipo y estado |
| `POST /plantas`, `POST /activos`, `POST /activos/consulta`, `POST /activos/actualizaciones-estado` | Demostración: si el cuerpo es el ejemplo del contrato, responden su ejemplo con `X-Inventario-Demo: true`; no modifican la base |
| `GET /health`, `GET /openapi.yaml` | Salud y contrato |

## 4. Cómo implementar una historia de usuario

Ejemplo: `POST /activos` (registrar un activo), que hoy responde el ejemplo del contrato. Se escribe de abajo hacia arriba y cada paso tiene su prueba. El catálogo (`catalogo.go` en cada capa) sirve de modelo completo.

1. **Esquema** (si hace falta): una migración nueva `app/infrastructure/migraciones/003_....sql`. Al arrancar se aplica sola. Numerar siempre con tres dígitos y nunca editar una migración ya fusionada (`001` y `002` crean y llenan el catálogo): los cambios van en una nueva.
2. **Dominio**: en `app/business/domain`, los tipos y reglas que no dependen de nada externo (por ejemplo, `ActivoNuevo` y su validación). Sin etiquetas JSON ni SQL.
3. **Puerto**: en `app/business/services/puertos.go`, la interfaz que necesita el caso de uso:
   ```go
   type RepositorioActivos interface {
       Registrar(ctx context.Context, activo domain.Activo) (domain.Activo, error)
   }
   ```
4. **Repositorio**: `app/infrastructure/activos.go` implementa la interfaz con consultas parametrizadas (`$1`, nunca concatenando texto) y pasa cada error por `traducirError`. Así `sql.ErrNoRows` llega arriba como `domain.ErrNoEncontrado` y una clave repetida como `domain.ErrDuplicado`.
5. **Caso de uso**: `app/business/services/activos.go` recibe el puerto en su constructor (`NuevoActivos(repo RepositorioActivos)`), valida y devuelve errores de dominio: `domain.Invalido(...)`, `domain.NoEncontrado(...)` o `&domain.Error{Categoria: ..., Codigo: "PLANTA_DESCONOCIDA", ...}` si el contrato define un código propio.
6. **Manejador**: `app/api/activos.go` lee la petición (`r.PathValue`, `LeerJSON`), llama al servicio y responde con `EscribirJSON` o `ResponderError`. Los DTO con los nombres JSON del contrato (camelCase) viven aquí y se convierten desde el dominio. Se agrega el caso de uso a `Dependencias`, la ruta en `registrarRutas` (`servidor.go`) y se quita ese POST de la lista de `ejemplos.go`.
7. **Conexión**: en `app/main.go`, `services.NuevoActivos(infrastructure.Activos{DB: db})` y se pasa en `api.Dependencias`.
8. **Pruebas**:
   - servicio: con un doble del puerto (ver `app/business/services/catalogo_test.go`);
   - manejador: con `httptest` y el servicio armado sobre un doble (ver `app/api/catalogo_test.go`);
   - repositorio: de integración contra PostgreSQL con `INVENTARIO_TEST_DSN` (ver `app/infrastructure/catalogo_integration_test.go`, que usa un esquema propio y lo borra al terminar).

Para la HU de actualización por lotes, el pool va en `app/business/workers` según el diseño de `doc.go` (reparto por `hash(id) mod N` y `core.Config.Workers`). El servicio lo usa y le pasa la función que aplica un cambio con el repositorio (`SELECT ... FOR UPDATE`).

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

Requisitos: Go 1.26 o superior y Docker con el plugin Compose. Desde la raíz del repositorio:

```bash
cp .env.example .env                 # poner una POSTGRES_PASSWORD cualquiera
docker compose up -d --build --wait  # docker-compose.yaml + docker-compose.override.yaml
curl http://localhost:8080/health
curl http://localhost:8080/activos/PL-001-PS-07
curl "http://localhost:8080/plantas?limit=1"
curl "http://localhost:8080/activos?plantaId=PL-003"
curl http://localhost:8080/openapi.yaml
```

Pruebas y linter:

```bash
go test ./...                     # unitarias (sin base de datos)
# Con la base del Compose anterior levantada, también las de integración:
INVENTARIO_TEST_DSN="postgres://inventario:<POSTGRES_PASSWORD>@127.0.0.1:5433/inventario?sslmode=disable" go test ./...
golangci-lint run                 # el mismo linter del CI, con las reglas de capas
go run ./app                      # la API fuera de Docker (requiere las variables PG*)
```

Con la base detenida (`docker compose stop db`), `/health` responde 503; al iniciarla (`docker compose start db`) vuelve a responder 200 sin reiniciar la API.

## 7. Flujo de trabajo

Cada integrante trabaja en su rama, abre un PR hacia `develop` (lo revisa el CI) y `develop` se fusiona en `main` para desplegar. La plantilla del PR está en [.github/pull_request_template.md](.github/pull_request_template.md).

## 8. CI/CD y despliegue

**Integración continua** (`.github/workflows/ci.yml`), en cada Pull Request hacia `develop`:

1. Linter: `golangci-lint` con la configuración de `.golangci.yml` (errcheck, govet, staticcheck, ineffassign, unused, formato `gofmt` y reglas de capas con `depguard`).
2. Seguridad: Semgrep con las reglas OWASP Top 10, Go, secretos, Dockerfile, Compose y GitHub Actions.

**Despliegue continuo** (`.github/workflows/cd.yml`), en cada push a `main` (al fusionar `develop`) o a mano desde *Actions → CD → Run workflow*: se publica la imagen en GHCR (`ghcr.io/infraestructuras-voltrix/voltrix-inventario-api:<commit>` y `:latest`), que es **pública**: la EC2 y el clúster de Floci la descargan sin `docker login` ni secretos de registro. Luego se copia `docker-compose.yaml` a la EC2 por SSH, se reinician los contenedores y se verifica `/health`, que reporta el commit desplegado en `version`. Si la versión nueva no responde, el pipeline vuelve a la imagen que estaba corriendo y el job falla. Los cambios de esquema deben admitir la versión anterior de la API.

**Configuración única en GitHub** (la hace un administrador del repositorio):

1. *Settings → Branches*: regla para `develop` con "Require a pull request before merging" y los checks obligatorios **Linter (golangci-lint)** y **Análisis de seguridad (Semgrep)**; regla para `main` con "Require a pull request before merging" (solo se fusiona `develop`).
2. Secretos `EC2_HOST`, `EC2_USUARIO` y `EC2_LLAVE_SSH`: los carga `infra/ec2/desplegar.sh` (ver abajo). Solo se cargan a mano en *Settings → Secrets and variables → Actions* si la EC2 se creó de otra forma. Variable opcional: `EC2_DIRECTORIO` (por defecto `voltrix-inventario`, en el home del usuario).
3. Imagen pública (una sola vez, después del primer CD): un paquete nuevo de GHCR nace privado, así que el primer CD falla en el paso *Comprobar que la imagen es pública* con el enlace exacto. Un administrador de la organización abre [la configuración del paquete](https://github.com/orgs/infraestructuras-voltrix/packages/container/voltrix-inventario-api/settings), elige *Danger Zone → Change visibility → Public* y vuelve a ejecutar el workflow. La organización ya permite paquetes públicos.

**EC2 (IaC, un solo comando)**: [infra/ec2/desplegar.sh](infra/ec2/desplegar.sh) crea la EC2 con la plantilla [infra/ec2/inventario.yaml](infra/ec2/inventario.yaml) y la conecta al CD sin abrir la consola de AWS ni la de GitHub:

```bash
infra/ec2/desplegar.sh                          # región de la AWS CLI
AWS_REGION=us-east-1 infra/ec2/desplegar.sh     # otra región
```

Requisitos: AWS CLI v2 con credenciales (`aws configure`), GitHub CLI autenticada (`gh auth login`) con permiso de administrador en el repositorio, y `ssh`. En Windows se ejecuta desde Git Bash o WSL.

| Paso | Qué hace |
|---|---|
| Parámetros | Lee [infra/ec2/parametros.env](infra/ec2/parametros.env) (`t3.micro`, API y SSH abiertos). Si `VpcId` y `PublicSubnet` están vacíos, usa la VPC por defecto y una subred en una zona donde exista el tipo de instancia. |
| Pila | `aws cloudformation deploy`: instancia, grupo de seguridad (solo 22 y 8080; PostgreSQL no se publica), llave SSH del pipeline (la privada queda en Parameter Store) e IP elástica, que no cambia al reiniciar. |
| Arranque | Espera a que la instancia termine de configurar swap de 1 GiB, Docker con Compose, el usuario `ubuntu` en el grupo `docker` y `~/voltrix-inventario/.env` con una contraseña generada en la instancia. |
| Secretos | Carga `EC2_HOST`, `EC2_USUARIO` y `EC2_LLAVE_SSH` en el repositorio con `gh secret set`. |
| Despliegue | Lanza el workflow CD sobre `main` (si `cd.yml` ya está en `main`). |

Se puede ejecutar de nuevo sin riesgo: solo cambia la pila si cambió la plantilla o los parámetros. Para eliminar la instancia, la IP y la llave: `aws cloudformation delete-stack --stack-name voltrix-inventario`. Esto borra también los datos de la base, que viven en el disco de la instancia.

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
$activo = Invoke-RestMethod "$base/activos/PL-001-PS-07" -TimeoutSec 10
if ($activo.id -ne 'PL-001-PS-07' -or $activo.plantaId -ne 'PL-001') { throw 'Activo inesperado' }
Test-NetConnection ([uri]$base).Host -Port 5432 -InformationLevel Quiet   # debe dar False
```

PostgreSQL no se publica fuera de la EC2, por eso el puerto 5432 debe dar `False`.
