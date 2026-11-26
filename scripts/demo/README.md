# Demo del Inventario de Activos

Arnés de demostración de la tarjeta T-INV-09. Equipos falsos (Telemetría, Reportes, Pronóstico y
Mantenimiento) consultan la API con su `X-Equipo-Consumidor`, y cada criterio de aceptación se
comprueba contra el servicio real (✓/✗). Se ejecuta desde la raíz del repositorio.

## Instalar

```bash
python -m venv .venv && source .venv/bin/activate
pip install -r scripts/demo/requirements.txt
```

## En local (docker compose)

```bash
cp .env.example .env   # y completar POSTGRES_PASSWORD
docker compose up -d --build --wait

python -m scripts.demo                          # guion por escenas con los equipos falsos
python -m scripts.demo.criterios --hu 01        # HU-INV-01: plantas
python -m scripts.demo.criterios --hu 02        # HU-INV-02: activos
python -m scripts.demo.criterios --hu 01 --solo 3,4
```

Con el compose local, la demo muestra la evidencia en PostgreSQL (`docker compose exec db psql`),
lee los registros de la API y, en el criterio del 503, detiene `db` y la vuelve a iniciar al
terminar. Use `--sin-parar-bd` para saltarse ese criterio. Si el compose usa otro proyecto u
otros archivos, indíquelos con `--compose-proyecto NOMBRE` y `--compose-archivo RUTA` (se puede
repetir, por ejemplo para un override de puertos).

## Contra EC2

```bash
python -m scripts.demo.criterios --hu 02 --url http://<ip-ec2>:8080
DEMO_URL=http://<ip-ec2>:8080 python -m scripts.demo
```

Fuera de local no hay acceso a la base de datos, así que la evidencia en PostgreSQL y el criterio
del 503 aparecen como "omitido", con su motivo, y no cuentan como fallo.

Códigos de salida: `0` si todo sale ✓, `1` si alguna comprobación da ✗ y `2` si no se pudo
ejecutar (la API no responde o se pidió una historia aún no implementada).
