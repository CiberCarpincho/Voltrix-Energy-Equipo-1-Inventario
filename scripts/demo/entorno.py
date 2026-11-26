"""Acceso al entorno local de Docker: SQL de evidencia, parar/iniciar PostgreSQL y registros.

La demo por criterios necesita mirar dentro del servicio (la base de datos y los registros) y,
para el criterio de PostgreSQL caído, detener la base un momento. Con el `docker compose` local
se hace con `docker compose -p <proyecto> -f <archivo> ...`. Si la API está en otra máquina
(EC2) o no hay Docker, el entorno queda "no disponible" y esos criterios se omiten.
"""

import os
import shlex
import subprocess
import time
from collections.abc import Sequence
from pathlib import Path
from typing import Protocol
from urllib.parse import urlsplit

_HOSTS_LOCALES = ("localhost", "127.0.0.1", "::1")


class Entorno(Protocol):
    """Lo que la demo pide al entorno; en las pruebas lo reemplaza una versión en proceso."""

    disponible: bool
    motivo: str  # por qué no está disponible ("" si lo está)

    def texto(self, *args: str) -> str:
        """El comando `docker compose ...` tal como se escribe en la terminal, para mostrarlo."""
        ...

    def texto_sql(self, consulta: str) -> str: ...

    def sql(self, consulta: str) -> list[list[str]]: ...

    def registros(self) -> str: ...

    def parar_bd(self) -> bool: ...

    def iniciar_bd(self) -> bool: ...


class DockerLocal:
    def __init__(
        self,
        raiz: Path,
        proyecto: str,
        archivos: Sequence[str],
        url: str,
        servicio: str = "api",
        base: str = "db",
    ) -> None:
        self._raiz = raiz
        self._prefijo = ["-p", proyecto, *[x for a in archivos for x in ("-f", a)]]
        self._servicio = servicio
        self._base = base
        self.motivo = self._detectar(url, archivos)
        self.disponible = not self.motivo

    def _detectar(self, url: str, archivos: Sequence[str]) -> str:
        if urlsplit(url).hostname not in _HOSTS_LOCALES:
            return "la API no es local; el docker compose de esta máquina no es el que la atiende"
        faltan = [a for a in archivos if not (self._raiz / a).exists()]
        if faltan:
            return f"no existe el archivo de compose {', '.join(faltan)}"
        try:
            r = self._compose("ps", "-q", self._servicio, espera_s=15)
        except FileNotFoundError:
            return "no hay docker instalado en esta máquina"
        except (OSError, subprocess.TimeoutExpired) as error:
            return f"docker compose no responde ({error})"
        if r.returncode != 0:
            detalle = (r.stderr.strip().splitlines() or ["sin detalle"])[-1]
            return f"docker compose falló: {detalle[:160]}"
        if not r.stdout.strip():
            return f"el servicio «{self._servicio}» no está corriendo en el proyecto {self._prefijo[1]}"
        return ""

    def _compose(
        self, *args: str, entrada: str | None = None, espera_s: float = 60
    ) -> subprocess.CompletedProcess[str]:
        # El compose exige POSTGRES_PASSWORD solo para interpolar el archivo. Para ps, exec,
        # stop, start y logs no se usa (no se recrean contenedores), así que basta un valor
        # cualquiera si no está definida.
        entorno = {**os.environ}
        entorno.setdefault("POSTGRES_PASSWORD", "no-usada-por-la-demo")
        return subprocess.run(  # noqa: S603 - comando fijo, sin entrada del usuario
            ["docker", "compose", *self._prefijo, *args],
            cwd=self._raiz, input=entrada, capture_output=True, text=True,
            check=False, timeout=espera_s, env=entorno,
        )  # fmt: skip

    def texto(self, *args: str) -> str:
        return shlex.join(["docker", "compose", *self._prefijo, *args])

    def texto_sql(self, consulta: str) -> str:
        return (
            self.texto("exec", "-T", self._base, "psql", "-U", "inventario", "-d", "inventario")
            + f" -c {shlex.quote(consulta)}"
        )

    def sql(self, consulta: str) -> list[list[str]]:
        r = self._compose(
            "exec", "-T", self._base,
            "psql", "-U", "inventario", "-d", "inventario", "-tA", "-F", "|", "-v", "ON_ERROR_STOP=1",
            entrada=consulta,
        )  # fmt: skip
        if r.returncode != 0:
            raise RuntimeError(f"La consulta falló: {r.stderr.strip()[:200]}")
        return [linea.split("|") for linea in r.stdout.splitlines() if linea]

    def registros(self) -> str:
        r = self._compose("logs", "--no-color", "--tail", "2000", self._servicio)
        return r.stdout + r.stderr

    def parar_bd(self) -> bool:
        return self._compose("stop", self._base).returncode == 0

    def iniciar_bd(self) -> bool:
        if self._compose("start", self._base).returncode != 0:
            return False
        for _ in range(30):  # espera a que acepte consultas
            try:
                self.sql("select 1")
                return True
            except (RuntimeError, subprocess.TimeoutExpired):
                time.sleep(2)
        return False
