"""Guion del demo: cada escena cuenta qué pasa, lo hace de verdad contra la API y lo comprueba.

Solo usa las operaciones que hoy consultan PostgreSQL de verdad (GET de plantas y activos,
/health y /openapi.yaml). Los POST siguen siendo respuestas de demostración del contrato.
"""

import sys
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import Any

import httpx

from scripts.demo.configuracion import Configuracion, cargar
from scripts.demo.entorno import DockerLocal, Entorno
from scripts.demo.equipos_falsos import ClienteApi, Mantenimiento, Pronostico, Reportes, Telemetria
from scripts.demo.salida import Respuesta, Salida

# Códigos de salida del demo
CORRECTO, COMPROBACION_FALLIDA, SIN_EJECUTAR = 0, 1, 2


def _cuerpo(r: Respuesta) -> dict[str, Any]:
    return r.cuerpo if isinstance(r.cuerpo, dict) else {}


def _datos(r: Respuesta) -> list[dict[str, Any]]:
    datos = _cuerpo(r).get("datos")
    return datos if isinstance(datos, list) else []


def correr(config: Configuracion, salida: Salida, http: httpx.Client, entorno: Entorno) -> bool | None:
    """True si todo salió ✓, False si algo falló y None si la API no está lista para la demo."""
    api = ClienteApi(http)
    telemetria = Telemetria(api, salida)
    reportes = Reportes(api, salida)
    pronostico = Pronostico(api, salida)
    mantenimiento = Mantenimiento(api, salida)

    salida.titulo("Inventario de Activos · Demo con equipos falsos")
    salida.linea(f"   API: {config.url}")
    r = api.enviar("GET", "/health")
    if r.estado != 200:
        salida.linea(f"   ✗ La API no responde como se espera (GET /health → {r.estado})")
        return None
    contrato = api.enviar("GET", "/openapi.yaml").cuerpo
    if isinstance(contrato, dict):  # el contrato se publica en JSON (válido como YAML)
        salida.linea(f"   {contrato['info']['title']} · contrato {contrato['info']['version']}")
    if entorno.disponible:
        salida.linea(f"   docker compose local: {entorno.texto('ps')}")
    else:
        salida.linea(f"   docker compose local: omitido ({entorno.motivo})")
    salida.pausa("Enter para empezar")

    # --- 1
    salida.escena(1, "Reportes y Pronóstico recorren el catálogo",
                  "Reportes lista las plantas página a página; Pronóstico mira una planta y sus activos.")  # fmt: skip
    plantas: list[dict[str, Any]] = []
    offset, ok_paginas = 0, True
    while True:
        r = reportes.listar_plantas(limit=50, offset=offset)
        ok_paginas &= salida.respuesta(r, 200, f"Página de plantas desde la posición {offset}.")
        plantas.extend(_datos(r))
        offset += 50
        if r.estado != 200 or offset >= int(_cuerpo(r).get("total", 0)):
            break
    total = _cuerpo(r).get("total")
    salida.registrar("Escena 1", f"Reportes recorrió {len(plantas)} de {total} plantas", ok_paginas and len(plantas) == total == 80)  # fmt: skip
    r = pronostico.listar_plantas(tipo="eolica", limit=5)
    ok = salida.respuesta(r, 200, "Pronóstico filtra las plantas eólicas: el viento se pronostica aparte.")
    salida.registrar("Escena 1", "Pronóstico filtra por tipo (eolica → 25)", ok and _cuerpo(r).get("total") == 25)  # fmt: skip
    r = pronostico.consultar_planta("PL-002")
    ok = salida.respuesta(r, 200, "El detalle trae el resumen de activos por tipo y por estado.")
    resumen = _cuerpo(r).get("resumen") or {}
    r = pronostico.activos_de_planta("PL-002")
    ok2 = salida.respuesta(r, 200, "Los activos de la planta, para estimar su generación.")
    salida.registrar("Escena 1", "el resumen de PL-002 coincide con sus activos", ok and ok2 and resumen.get("totalActivos") == _cuerpo(r).get("total") == len(_datos(r)))  # fmt: skip
    r = reportes.activos_de_planta("PL-001", tipo="panel_solar")
    ok = salida.respuesta(r, 200, "Reportes cuenta los paneles de PL-001 para su reporte de generación.")
    salida.registrar("Escena 1", "Reportes pide los paneles de PL-001 (20)", ok and _cuerpo(r).get("total") == 20)  # fmt: skip
    salida.pausa()

    # --- 2
    salida.escena(2, "Mantenimiento busca activos en mantenimiento",
                  "Mantenimiento Predictivo filtra por estado y consulta cada activo.")  # fmt: skip
    r = mantenimiento.activos_por_estado("en_mantenimiento")
    ok = salida.respuesta(r, 200, "Los activos que hoy están en mantenimiento.")
    ids = [str(a.get("id")) for a in _datos(r)]
    salida.registrar("Escena 2", f"hay 2 activos en mantenimiento ({', '.join(ids)})", ok and _cuerpo(r).get("total") == 2)  # fmt: skip
    for id_ in ids:
        r = mantenimiento.consultar_activo(id_)
        ok = salida.respuesta(r, 200, "Detalle del activo, con su planta y la fecha del último cambio.")
        salida.registrar("Escena 2", f"{id_} sigue en_mantenimiento", ok and _cuerpo(r).get("estado") == "en_mantenimiento")  # fmt: skip
    r = telemetria.consultar_activo("PL-002-TB-01")
    ok = salida.respuesta(r, 200, "Telemetría consulta la misma turbina antes de reportar su lectura.")
    salida.registrar("Escena 2", "Telemetría consulta PL-002-TB-01", ok and _cuerpo(r).get("plantaId") == "PL-002")  # fmt: skip
    # TODO(T-INV-09): escenas de lotes de Telemetría. Cuando existan los endpoints reales
    # (HU-06 a HU-10), agregar aquí:
    #   - telemetria.consultar_lote([...])  → POST /activos/consulta (encontrados y desconocidos)
    #   - telemetria.actualizar_estados([...]) → POST /activos/actualizaciones-estado
    #     (aplicado, ignorado por fecha antigua, desconocido) y volver a consultar el activo
    #     para ver el estado nuevo.
    # Hoy ambos son stubs de demo (X-Inventario-Demo: true) y no cambian la base.
    salida.nota("Las escenas de lotes de Telemetría (consulta y actualizaciones-estado) se agregan")
    salida.nota("cuando esos endpoints dejen de ser respuestas de demostración.")
    salida.pausa()

    # --- 3
    salida.escena(3, "Lo que el servicio rechaza",
                  "Filtros fuera del contrato e identificadores que no existen.")  # fmt: skip
    for equipo, ruta, esperado, codigo, texto in (
        (reportes, "/plantas?tipo=nuclear", 400, "SOLICITUD_INVALIDA", "Ese tipo de planta no existe en el contrato."),
        (mantenimiento, "/activos?estado=roto", 400, "SOLICITUD_INVALIDA", "Ese estado no existe en el contrato."),
        (reportes, "/activos?limit=500", 400, "SOLICITUD_INVALIDA", "El tamaño de página máximo es 200."),
        (pronostico, "/plantas/PL-999", 404, "NO_ENCONTRADO", "La planta no está en el catálogo."),
        (telemetria, "/activos/PL-001-PS-99", 404, "NO_ENCONTRADO", "El activo no está en el catálogo."),
    ):  # fmt: skip
        r = equipo.pedir("GET", ruta)
        ok = salida.respuesta(r, esperado, texto)
        salida.registrar("Escena 3", f"GET {ruta} → {esperado} {codigo}", ok and _cuerpo(r).get("codigo") == codigo and bool(_cuerpo(r).get("requestId")))  # fmt: skip
    salida.pausa()

    # --- 4
    salida.escena(4, "Salud del servicio",
                  "Lo que miran el despliegue continuo y los balanceadores antes de enviar tráfico.")  # fmt: skip
    salida.solicitud("Operación", "GET", "/health", None)
    r = api.enviar("GET", "/health")
    ok = salida.respuesta(r, 200, "La API responde y confirma la conexión con PostgreSQL.")
    salida.registrar("Escena 4", "GET /health → 200 con baseDatos ok", ok and _cuerpo(r).get("baseDatos") == "ok")  # fmt: skip
    salida.registrar("Escena 4", "toda respuesta trae X-Request-Id", bool(r.cabeceras.get("x-request-id")))  # fmt: skip

    if entorno.disponible:
        salida.linea()
        salida.comando(entorno.texto("logs", "--tail", "20", "api"), "Registros de la API")
        lineas = [x for x in entorno.registros().splitlines() if x.strip()][-20:]
        salida.evidencia("Últimas líneas del registro de la API:", lineas or ["(la API no escribió líneas en su registro)"])  # fmt: skip
    return salida.resumen()


def _ejecutar(config: Configuracion, salida: Salida, http: httpx.Client | None, entorno: Entorno | None) -> int:  # fmt: skip
    cliente = http or httpx.Client(base_url=config.url, timeout=15)
    entorno = entorno or DockerLocal(Path.cwd(), config.compose_proyecto, config.compose_archivos, config.url)  # fmt: skip
    codigo = CORRECTO
    try:
        resultado = correr(config, salida, cliente, entorno)
        if resultado is None:
            codigo = SIN_EJECUTAR
        elif not resultado:
            codigo = COMPROBACION_FALLIDA
    except httpx.TransportError:
        salida.linea(f"✗ No se pudo conectar a {config.url}. ¿Está levantada la API? "
                     "(docker compose -f deploy/compose/compose.yaml up -d --build --wait)")  # fmt: skip
        codigo = SIN_EJECUTAR
    finally:
        if http is None:
            cliente.close()
    return codigo


def main(
    argv: Sequence[str] | None = None,
    http: httpx.Client | None = None,
    entorno: Entorno | None = None,
    escribir: Callable[[str], None] = print,
    leer: Callable[[str], str] = input,
) -> int:
    config = cargar(sys.argv[1:] if argv is None else argv)
    salida = Salida(
        color=config.color and sys.stdout.isatty(),
        pausar=not config.rapido and sys.stdin.isatty(),
        escribir=escribir,
        leer=leer,
    )
    return _ejecutar(config, salida, http, entorno)
