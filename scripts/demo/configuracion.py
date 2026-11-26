"""Configuración del demo: argumentos y variables de entorno.

Inventario no usa API Keys ni colas: basta con la dirección de la API y, para la evidencia en la
base de datos y las pruebas con PostgreSQL caído, saber qué `docker compose` local la levanta.
"""

import argparse
import os
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

URL_POR_DEFECTO = "http://localhost:8080"
PROYECTO_POR_DEFECTO = "voltrix-inventario"  # el `name:` de docker-compose.yaml
# El mismo par que `docker compose up` toma por defecto en desarrollo (base + override).
ARCHIVOS_COMPOSE_POR_DEFECTO = ("docker-compose.yaml", "docker-compose.override.yaml")


@dataclass(frozen=True)
class Configuracion:
    url: str
    rapido: bool
    color: bool
    compose_proyecto: str
    compose_archivos: tuple[str, ...]


def crear_parser(*, add_help: bool = True) -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="python -m scripts.demo",
        description="Demo: equipos falsos consultan el catálogo del Inventario de Activos.",
        add_help=add_help,
    )
    parser.add_argument(
        "--url", help=f"Dirección de la API (variable DEMO_URL; por defecto {URL_POR_DEFECTO})"
    )
    parser.add_argument("--rapido", action="store_true", help="Sin pausas entre escenas")
    parser.add_argument("--sin-color", action="store_true", help="Salida sin colores")
    parser.add_argument(
        "--compose-proyecto",
        default=PROYECTO_POR_DEFECTO,
        help=f"Proyecto del docker compose local (por defecto {PROYECTO_POR_DEFECTO})",
    )
    parser.add_argument(
        "--compose-archivo",
        action="append",
        default=None,
        help=f"Archivo de compose; se puede repetir (por defecto {" y ".join(ARCHIVOS_COMPOSE_POR_DEFECTO)})",
    )
    return parser


def configuracion_de(
    args: argparse.Namespace, entorno: Mapping[str, str] | None = None
) -> Configuracion:
    """Arma la configuración. Prioridad de la URL: `--url`, `DEMO_URL`, valor por defecto."""
    entorno = os.environ if entorno is None else entorno
    return Configuracion(
        url=(args.url or entorno.get("DEMO_URL") or URL_POR_DEFECTO).rstrip("/"),
        rapido=args.rapido,
        color=not args.sin_color and "NO_COLOR" not in entorno,
        compose_proyecto=args.compose_proyecto,
        compose_archivos=tuple(args.compose_archivo or ARCHIVOS_COMPOSE_POR_DEFECTO),
    )


def cargar(argv: Sequence[str], entorno: Mapping[str, str] | None = None) -> Configuracion:
    return configuracion_de(crear_parser().parse_args(argv), entorno)
