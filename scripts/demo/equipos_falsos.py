"""Los equipos falsos: consultan el Inventario como lo harían sus servicios reales.

Cada equipo se identifica con `X-Equipo-Consumidor` (contrato, parámetro EquipoConsumidor) y
expone solo las operaciones del contrato que ese equipo usa según la tabla de consumidores.
"""

import time
from typing import Any
from urllib.parse import urlencode

import httpx

from scripts.demo.salida import Respuesta, Salida


def ruta_con_filtros(base: str, **filtros: Any) -> str:
    """`/plantas` + `?tipo=solar&limit=10`, omitiendo los filtros en None."""
    valores = {k: v for k, v in filtros.items() if v is not None}
    return f"{base}?{urlencode(valores)}" if valores else base


class ClienteApi:
    """Hace las peticiones HTTP y mide cuánto tarda cada una."""

    def __init__(self, http: httpx.Client) -> None:
        self._http = http

    def enviar(
        self,
        metodo: str,
        ruta: str,
        equipo: str | None = None,
        cuerpo: dict[str, Any] | None = None,
    ) -> Respuesta:
        cabeceras = {"X-Equipo-Consumidor": equipo} if equipo else {}
        inicio = time.monotonic()
        r = self._http.request(metodo, ruta, json=cuerpo, headers=cabeceras)
        ms = (time.monotonic() - inicio) * 1000
        try:
            contenido: Any = r.json()
        except ValueError:
            contenido = r.text or None
        return Respuesta(r.status_code, contenido, ms, dict(r.headers))


class EquipoFalso:
    def __init__(self, nombre: str, codigo: str, api: ClienteApi, salida: Salida) -> None:
        self.nombre = nombre
        self.codigo = codigo  # valor de X-Equipo-Consumidor, p. ej. "equipo-2"
        self._api = api
        self._salida = salida

    @property
    def etiqueta(self) -> str:
        return f"{self.nombre} ({self.codigo})"

    def pedir(self, metodo: str, ruta: str, cuerpo: dict[str, Any] | None = None) -> Respuesta:
        """Muestra la petición, la envía con la cabecera del equipo y devuelve la respuesta."""
        self._salida.solicitud(self.etiqueta, metodo, ruta, cuerpo)
        return self._api.enviar(metodo, ruta, self.codigo, cuerpo)


class Telemetria(EquipoFalso):
    """Equipo 2: `POST /activos/consulta`, `POST /activos/actualizaciones-estado`, `GET /activos/{id}`."""

    def __init__(self, api: ClienteApi, salida: Salida) -> None:
        super().__init__("Telemetría", "equipo-2", api, salida)

    def consultar_activo(self, activo_id: str) -> Respuesta:
        return self.pedir("GET", f"/activos/{activo_id}")

    def consultar_lote(self, ids: list[str]) -> Respuesta:
        # Hoy es un stub de demo (X-Inventario-Demo: true): solo acepta el cuerpo de ejemplo.
        return self.pedir("POST", "/activos/consulta", {"ids": ids})

    def actualizar_estados(self, cambios: list[dict[str, Any]]) -> Respuesta:
        # Hoy es un stub de demo (X-Inventario-Demo: true): solo acepta el cuerpo de ejemplo.
        return self.pedir("POST", "/activos/actualizaciones-estado", {"cambios": cambios})


class Reportes(EquipoFalso):
    """Equipo 4: `GET /plantas`, `GET /activos?plantaId=`."""

    def __init__(self, api: ClienteApi, salida: Salida) -> None:
        super().__init__("Reportes", "equipo-4", api, salida)

    def listar_plantas(self, **filtros: Any) -> Respuesta:
        return self.pedir("GET", ruta_con_filtros("/plantas", **filtros))

    def activos_de_planta(self, planta_id: str, **filtros: Any) -> Respuesta:
        return self.pedir("GET", ruta_con_filtros("/activos", plantaId=planta_id, **filtros))


class Pronostico(EquipoFalso):
    """Equipo 5: `GET /plantas`, `GET /plantas/{plantaId}`, `GET /activos?plantaId=`."""

    def __init__(self, api: ClienteApi, salida: Salida) -> None:
        super().__init__("Pronóstico", "equipo-5", api, salida)

    def listar_plantas(self, **filtros: Any) -> Respuesta:
        return self.pedir("GET", ruta_con_filtros("/plantas", **filtros))

    def consultar_planta(self, planta_id: str) -> Respuesta:
        return self.pedir("GET", f"/plantas/{planta_id}")

    def activos_de_planta(self, planta_id: str, **filtros: Any) -> Respuesta:
        return self.pedir("GET", ruta_con_filtros("/activos", plantaId=planta_id, **filtros))


class Mantenimiento(EquipoFalso):
    """Equipo 6: `GET /activos?estado=`, `GET /activos/{id}`."""

    def __init__(self, api: ClienteApi, salida: Salida) -> None:
        super().__init__("Mantenimiento", "equipo-6", api, salida)

    def activos_por_estado(self, estado: str, **filtros: Any) -> Respuesta:
        return self.pedir("GET", ruta_con_filtros("/activos", estado=estado, **filtros))

    def consultar_activo(self, activo_id: str) -> Respuesta:
        return self.pedir("GET", f"/activos/{activo_id}")
