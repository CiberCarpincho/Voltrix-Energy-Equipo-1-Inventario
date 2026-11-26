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

### Despliegue continuo en EC2

Cada minuto, un temporizador de systemd consulta `main` en GitHub. Cuando cambia el commit, construye la API, aplica Compose y exige que `/health` confirme PostgreSQL. Si falla, recupera el commit y la imagen anteriores. El volumen y el archivo de credenciales pertenecen al despliegue existente. Las modificaciones de esquema de base de datos deben admitir la versión anterior de la API.

La plantilla instala el temporizador en instancias nuevas usando una revisión que incluya esta funcionalidad. Para activar la instancia ya desplegada, después de fusionar el cambio a `main`, ejecuta este bloque en la terminal de **Ubuntu EC2**. Usa una copia temporal del instalador para mantener el commit del servicio en ejecución hasta que comience la actualización.

```bash
cd ~/Voltrix-Energy-Equipo-1-Inventario
git fetch origin main &&
  cd_setup=$(mktemp -d) &&
  git archive FETCH_HEAD infra/ec2/cd | tar -x -C "$cd_setup" &&
  sudo bash "$cd_setup/infra/ec2/cd/instalar.sh" "$PWD"
```

Para comprobar el resultado:

```bash
systemctl is-active voltrix-cd.timer
sudo journalctl -u voltrix-cd.service -n 40 --no-pager
git rev-parse HEAD
git ls-remote origin refs/heads/main
curl -fsS http://localhost:8080/health
```

El temporizador debe estar activo, los dos SHA deben coincidir y la salud debe indicar `disponible` y `ok`. Para pausar los intentos ante una versión defectuosa, usa `sudo systemctl disable --now voltrix-cd.timer`. Reactiva con `sudo systemctl enable --now voltrix-cd.timer`. Si una revisión cambia el propio instalador o el controlador de despliegue, vuelve a ejecutar el bloque de instalación para actualizar la copia administrada por systemd.
