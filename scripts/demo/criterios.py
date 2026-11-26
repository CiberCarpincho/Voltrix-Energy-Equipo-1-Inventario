"""Demo por criterios de aceptación: recorre una historia criterio por criterio.

    python -m scripts.demo.criterios --hu 01               # HU-INV-01: consultar plantas
    python -m scripts.demo.criterios --hu 02               # HU-INV-02: consultar activos
    python -m scripts.demo.criterios --hu 01 --solo 3,4    # solo algunos criterios
    python -m scripts.demo.criterios --hu 02 --url http://<ip-ec2>:8080 --sin-parar-bd

Cada criterio muestra su texto del backlog, el curl equivalente, la respuesta real del servicio
y, con el `docker compose` local, la evidencia en PostgreSQL. Termina con un resumen de ✓ y ✗.
Códigos de salida: 0 todo ✓, 1 alguna comprobación ✗, 2 no se pudo ejecutar.
"""

import argparse
import re
import sys
import time
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import httpx

from scripts.demo.configuracion import Configuracion, configuracion_de, crear_parser
from scripts.demo.entorno import DockerLocal, Entorno
from scripts.demo.equipos_falsos import (
    ClienteApi,
    EquipoFalso,
    Mantenimiento,
    Pronostico,
    Reportes,
    Telemetria,
)
from scripts.demo.salida import Respuesta, Salida

CORRECTO, COMPROBACION_FALLIDA, SIN_EJECUTAR = 0, 1, 2
CAMPOS_PLANTA = ("id", "nombre", "tipo", "clienteId", "capacidadInstaladaKw", "creadoEn")
TIPOS_ACTIVO = ("panel_solar", "turbina", "banco_baterias")
ESTADOS_ACTIVO = ("operativo", "en_mantenimiento", "fuera_de_servicio")
# Semilla db/semillas/001_catalogo.sql
TOTAL_PLANTAS, TOTAL_ACTIVOS = 80, 1270
PLANTAS_POR_TIPO = {"solar": 40, "eolica": 25, "hibrida": 15}
ESPERA_SALUD_S = 60


@dataclass
class Contexto:
    config: Configuracion
    salida: Salida
    api: ClienteApi
    entorno: Entorno
    solo: set[int]
    telemetria: Telemetria
    reportes: Reportes
    pronostico: Pronostico
    mantenimiento: Mantenimiento
    parar_bd: bool = True
    dormir: Callable[[float], None] = time.sleep

    def toca(self, criterio: int) -> bool:
        return not self.solo or criterio in self.solo

    def curl(self, metodo: str, ruta: str, equipo: str | None) -> None:
        """Muestra el curl equivalente, con `$DEMO_URL` para que se pueda copiar y pegar."""
        partes = [f'curl -s{"" if metodo == "GET" else f" -X {metodo}"} "$DEMO_URL{ruta}"']
        if equipo:
            partes.append(f'-H "X-Equipo-Consumidor: {equipo}"')
        self.salida.comando(" \\\n  ".join(partes), "curl equivalente")

    def http(
        self, equipo: EquipoFalso, metodo: str, ruta: str, esperado: int, significado: str
    ) -> tuple[Respuesta, bool]:
        self.curl(metodo, ruta, equipo.codigo)
        r = equipo.pedir(metodo, ruta)
        return r, self.salida.respuesta(r, esperado, significado)

    def evidencia_sql(self, escena: str, titulo: str, consulta: str, esperado: str) -> None:
        """Cuenta en PostgreSQL y lo compara con lo que respondió la API (omitido sin compose)."""
        if not self.entorno.disponible:
            self.salida.omitir(escena, f"evidencia en PostgreSQL: {titulo}", self.entorno.motivo)
            return
        self.salida.comando(self.entorno.texto_sql(consulta), "Evidencia en la base de datos")
        try:
            filas = self.sql(consulta)
        except RuntimeError as error:
            self.salida.comprobar(escena, f"evidencia en PostgreSQL: {error}", False)
            return
        valor = filas[0][0] if filas else "(sin filas)"
        self.salida.evidencia("PostgreSQL responde:", [valor])
        self.salida.comprobar(escena, f"PostgreSQL confirma {titulo} = {esperado}", valor == esperado)

    def sql(self, consulta: str) -> list[list[str]]:
        return self.entorno.sql(consulta)


# ---------------------------------------------------------------------------------------------
# Ayudas para leer las respuestas
# ---------------------------------------------------------------------------------------------
def _cuerpo(r: Respuesta) -> dict[str, Any]:
    return r.cuerpo if isinstance(r.cuerpo, dict) else {}


def _datos(r: Respuesta) -> list[dict[str, Any]]:
    datos = _cuerpo(r).get("datos")
    return datos if isinstance(datos, list) else []


def _ids(r: Respuesta) -> list[str]:
    return [str(x.get("id")) for x in _datos(r)]


def _ordenados(ids: list[str]) -> bool:
    return ids == sorted(ids)


def _todas(c: Contexto, base: str, **filtros: str) -> list[dict[str, Any]]:
    """Recorre todas las páginas (sin mostrarlas) para comparar contra el catálogo completo."""
    elementos: list[dict[str, Any]] = []
    offset = 0
    while True:
        consulta = "&".join(f"{k}={v}" for k, v in {**filtros, "limit": "200", "offset": str(offset)}.items())  # fmt: skip
        r = c.api.enviar("GET", f"{base}?{consulta}", "equipo-1")
        pagina = _datos(r)
        elementos.extend(pagina)
        offset += 200
        if r.estado != 200 or not pagina or offset >= int(_cuerpo(r).get("total", 0)):
            return elementos


def _comprobar_error(c: Contexto, escena: str, r: Respuesta, codigo: str, que: str) -> None:
    """El cuerpo de error del contrato: codigo, mensaje y requestId (igual al X-Request-Id)."""
    cuerpo = _cuerpo(r)
    cabecera = r.cabeceras.get("x-request-id", "")
    completo = (
        cuerpo.get("codigo") == codigo
        and bool(cuerpo.get("mensaje"))
        and bool(cuerpo.get("requestId"))
        and cuerpo.get("requestId") == cabecera
    )
    c.salida.comprobar(escena, f"{que} → {r.estado} {codigo} con mensaje y requestId", completo and r.estado == {"SOLICITUD_INVALIDA": 400, "NO_ENCONTRADO": 404, "NO_DISPONIBLE": 503}[codigo])  # fmt: skip


def _pagina_por_defecto(c: Contexto, escena: str, r: Respuesta, total: int) -> None:
    cuerpo, ids = _cuerpo(r), _ids(r)
    s = c.salida
    s.comprobar(escena, f"trae {len(ids)} elementos (máximo 50)", 0 < len(ids) <= 50)
    s.comprobar(escena, "ordenados por id", _ordenados(ids))
    s.comprobar(escena, f"total = {cuerpo.get('total')} (se esperaba {total})", cuerpo.get("total") == total)  # fmt: skip
    s.comprobar(
        escena,
        f"limit = {cuerpo.get('limit')} y offset = {cuerpo.get('offset')} (se esperaba 50 y 0)",
        cuerpo.get("limit") == 50 and cuerpo.get("offset") == 0,
    )


def _paginacion(c: Contexto, escena: str, equipo: EquipoFalso, base: str, total: int) -> None:
    """Reglas comunes de limit y offset (HU-INV-01 criterio 3 y HU-INV-02 criterio 4)."""
    s = c.salida
    completo = [str(x.get("id")) for x in _todas(c, base)]
    for limite, offset in ((1, 0), (10, 20), (200, 0), (200, total - 5)):
        ruta = f"{base}?limit={limite}&offset={offset}"
        r, ok = c.http(equipo, "GET", ruta, 200, f"Página de hasta {limite} elementos desde la posición {offset}.")  # fmt: skip
        cuerpo, ids = _cuerpo(r), _ids(r)
        esperado = completo[offset : offset + limite]
        s.comprobar(
            escena,
            f"limit={limite}&offset={offset}: devuelve limit {cuerpo.get('limit')}, offset "
            f"{cuerpo.get('offset')} y {len(ids)} elementos (se esperaban {len(esperado)})",
            ok and cuerpo.get("limit") == limite and cuerpo.get("offset") == offset
            and ids == esperado and cuerpo.get("total") == total,
        )  # fmt: skip
    offset = total + 100
    r, ok = c.http(equipo, "GET", f"{base}?offset={offset}", 200, "Más allá del final: página vacía, no es un error.")  # fmt: skip
    cuerpo = _cuerpo(r)
    s.comprobar(
        escena,
        f"offset={offset} (> total {total}) → 200 con datos: [] y total {cuerpo.get('total')}",
        ok and cuerpo.get("datos") == [] and cuerpo.get("total") == total
        and cuerpo.get("offset") == offset,
    )  # fmt: skip


def _invalidos(c: Contexto, escena: str, equipo: EquipoFalso, casos: list[tuple[str, str]]) -> None:
    for ruta, que in casos:
        r, _ = c.http(equipo, "GET", ruta, 400, f"Rechazado: {que}.")
        _comprobar_error(c, escena, r, "SOLICITUD_INVALIDA", ruta)


def _esperar_salud(c: Contexto) -> bool:
    """Espera a que /health vuelva a 200 (la API recupera la conexión con PostgreSQL)."""
    limite = time.monotonic() + ESPERA_SALUD_S
    while time.monotonic() < limite:
        try:
            if c.api.enviar("GET", "/health").estado == 200:
                return True
        except httpx.TransportError:
            pass
        c.dormir(1)
    return False


def _bd_caida(c: Contexto, escena: str, numero: int, texto: str, rutas: list[tuple[EquipoFalso, str]]) -> None:  # fmt: skip
    """Detiene `db`, comprueba 503 NO_DISPONIBLE y SIEMPRE vuelve a iniciarla (en un finally)."""
    s = c.salida
    s.criterio(numero, texto)
    if not c.parar_bd:
        s.omitir(escena, "503 NO_DISPONIBLE con PostgreSQL caído", "se pidió --sin-parar-bd")
        return
    if not c.entorno.disponible:
        s.omitir(escena, "503 NO_DISPONIBLE con PostgreSQL caído", c.entorno.motivo)
        return
    s.nota("Falla provocada: se detiene PostgreSQL un momento.")
    s.comando(c.entorno.texto("stop", "db"), "Comando")
    if not c.entorno.parar_bd():
        s.comprobar(escena, "se pudo detener la base de datos", False)
        return
    vuelve = salud = False
    try:
        for equipo, ruta in rutas:
            r, _ = c.http(equipo, "GET", ruta, 503, "Sin base de datos: respuesta controlada, sin detalles internos.")  # fmt: skip
            _comprobar_error(c, escena, r, "NO_DISPONIBLE", f"GET {ruta} con la BD caída")
        r = c.api.enviar("GET", "/health")
        s.nota(f"GET /health con la BD caída → {r.estado} {_cuerpo(r)}")
    finally:
        s.nota("Se vuelve a iniciar PostgreSQL y se espera a que /health responda 200.")
        s.comando(c.entorno.texto("start", "db"), "Comando")
        vuelve = c.entorno.iniciar_bd()
        salud = vuelve and _esperar_salud(c)
    s.comprobar(escena, "PostgreSQL volvió a iniciar", vuelve)
    s.comprobar(escena, "GET /health volvió a 200", salud)
    equipo, ruta = rutas[0]
    r, ok = c.http(equipo, "GET", ruta, 200, "Con la base de vuelta, el servicio se recupera solo.")
    s.comprobar(escena, f"GET {ruta} vuelve a responder 200", ok)


# ---------------------------------------------------------------------------------------------
# HU-INV-01 · Consultar plantas
# ---------------------------------------------------------------------------------------------
def hu_01(c: Contexto) -> None:
    s = c.salida
    s.titulo("HU-INV-01 · Consultar el catálogo de plantas")
    s.nota("Como Reportes o Pronóstico requiero listar y consultar las plantas registradas,")
    s.nota("para conocer su tipo, cliente y capacidad sin mantener una copia propia.")
    if not c.entorno.disponible:
        s.nota(f"Sin el docker compose local se omite la evidencia en BD ({c.entorno.motivo}).")
    rep, pro = c.reportes, c.pronostico

    if c.toca(1):
        e = "HU-01 · Criterio 1"
        s.criterio(1, "Dado que el catálogo tiene plantas, cuando un consumidor envía GET /plantas sin "
                   "parámetros, entonces responde 200 con como máximo 50 plantas ordenadas por id, "
                   "total = 80, limit 50 y offset 0, y cada planta trae id, nombre, tipo, clienteId, "
                   "capacidadInstaladaKw y creadoEn.")  # fmt: skip
        r, ok = c.http(rep, "GET", "/plantas", 200, "Primera página del catálogo de plantas.")
        s.comprobar(e, "responde 200", ok)
        _pagina_por_defecto(c, e, r, TOTAL_PLANTAS)
        faltan = sorted({k for p in _datos(r) for k in CAMPOS_PLANTA if k not in p})
        s.comprobar(e, f"cada planta trae {', '.join(CAMPOS_PLANTA)}" + (f" (faltan: {', '.join(faltan)})" if faltan else ""), bool(_datos(r)) and not faltan)  # fmt: skip
        c.evidencia_sql(e, "el total de plantas", "select count(*) from plantas", str(TOTAL_PLANTAS))
        s.pausa()

    if c.toca(2):
        e = "HU-01 · Criterio 2"
        s.criterio(2, "Dado que se envían los filtros tipo y clienteId, cuando se combinan, entonces se "
                   "aplican con AND y total cuenta solo las plantas filtradas (solar 40, eolica 25, "
                   "hibrida 15).")  # fmt: skip
        for tipo, esperado in PLANTAS_POR_TIPO.items():
            r, ok = c.http(rep, "GET", f"/plantas?tipo={tipo}", 200, f"Solo plantas de tipo {tipo}.")
            total = _cuerpo(r).get("total")
            s.comprobar(e, f"tipo={tipo}: total {total} (se esperaba {esperado}) y todas son {tipo}", ok and total == esperado and all(p.get("tipo") == tipo for p in _datos(r)))  # fmt: skip
        todas = _todas(c, "/plantas")
        for tipo, cliente in (("solar", "CLI-DEMO-01"), ("eolica", "CLI-COOP-01")):
            esperado = sum(1 for p in todas if p.get("tipo") == tipo and p.get("clienteId") == cliente)  # fmt: skip
            del_cliente = sum(1 for p in todas if p.get("clienteId") == cliente)
            r, ok = c.http(pro, "GET", f"/plantas?tipo={tipo}&clienteId={cliente}", 200, f"Plantas {tipo} Y del cliente {cliente}.")  # fmt: skip
            total = _cuerpo(r).get("total")
            s.comprobar(
                e,
                f"tipo={tipo} AND clienteId={cliente}: total {total} (se esperaba {esperado}; el "
                f"cliente tiene {del_cliente} en total)",
                ok and total == esperado and esperado < del_cliente
                and all(p.get("tipo") == tipo and p.get("clienteId") == cliente for p in _datos(r)),
            )  # fmt: skip
            c.evidencia_sql(e, f"plantas {tipo} de {cliente}", f"select count(*) from plantas where tipo = '{tipo}' and cliente_id = '{cliente}'", str(esperado))  # fmt: skip
        s.pausa()

    if c.toca(3):
        e = "HU-01 · Criterio 3"
        s.criterio(3, "Dado que se envían limit (1 a 200) y offset (≥ 0), cuando la API responde, "
                   "entonces respeta y devuelve ambos valores; un offset mayor que total responde 200 "
                   "con datos: [].")  # fmt: skip
        _paginacion(c, e, rep, "/plantas", TOTAL_PLANTAS)
        s.pausa()

    if c.toca(4):
        e = "HU-01 · Criterio 4"
        s.criterio(4, "Dado que un parámetro es inválido (tipo=nuclear, limit=0, 201 o abc, offset=-1, "
                   "clienteId fuera de patrón), cuando la API lo valida, entonces responde 400 "
                   "SOLICITUD_INVALIDA con mensaje y requestId.")  # fmt: skip
        _invalidos(c, e, rep, [
            ("/plantas?tipo=nuclear", "tipo fuera del catálogo (solar, eolica, hibrida)"),
            ("/plantas?limit=0", "limit menor que 1"),
            ("/plantas?limit=201", "limit mayor que 200"),
            ("/plantas?limit=abc", "limit no es un número"),
            ("/plantas?offset=-1", "offset negativo"),
            ("/plantas?clienteId=CLI.COOP", "clienteId con un carácter fuera de ^[A-Za-z0-9_-]+$"),
            (f"/plantas?clienteId={'C' * 65}", "clienteId de más de 64 caracteres"),
        ])  # fmt: skip
        s.pausa()

    if c.toca(5):
        e = "HU-01 · Criterio 5"
        s.criterio(5, "Dado que la planta existe, cuando se envía GET /plantas/PL-001, entonces responde "
                   "200 con resumen (totalActivos, porTipo, porEstado) coherente con "
                   "GET /activos?plantaId=PL-001.")  # fmt: skip
        r, ok = c.http(pro, "GET", "/plantas/PL-001", 200, "Detalle de la planta con el resumen de sus activos.")  # fmt: skip
        resumen = _cuerpo(r).get("resumen") or {}
        s.comprobar(e, "responde 200 con id PL-001 y resumen", ok and _cuerpo(r).get("id") == "PL-001" and bool(resumen))  # fmt: skip
        ra, ok_a = c.http(pro, "GET", "/activos?plantaId=PL-001&limit=200", 200, "Los activos de la misma planta, para contrastar.")  # fmt: skip
        activos = _datos(ra)
        por_tipo = {t: sum(1 for a in activos if a.get("tipo") == t) for t in TIPOS_ACTIVO}
        por_estado = {x: sum(1 for a in activos if a.get("estado") == x) for x in ESTADOS_ACTIVO}
        s.evidencia("Contados a partir de GET /activos?plantaId=PL-001:", [
            f"total {_cuerpo(ra).get('total')}  ·  porTipo {por_tipo}", f"porEstado {por_estado}",
        ])  # fmt: skip
        s.comprobar(e, f"totalActivos {resumen.get('totalActivos')} = total de activos {_cuerpo(ra).get('total')} (22 en la semilla)", ok_a and resumen.get("totalActivos") == _cuerpo(ra).get("total") == len(activos) == 22)  # fmt: skip
        s.comprobar(e, f"porTipo coincide {resumen.get('porTipo')}", resumen.get("porTipo") == por_tipo)
        s.comprobar(e, f"porEstado coincide {resumen.get('porEstado')}", resumen.get("porEstado") == por_estado)  # fmt: skip
        s.pausa()

    if c.toca(6):
        e = "HU-01 · Criterio 6"
        s.criterio(6, "Dado que la planta no existe, cuando se envía GET /plantas/PL-999, entonces "
                   "responde 404 NO_ENCONTRADO.")  # fmt: skip
        r, _ = c.http(pro, "GET", "/plantas/PL-999", 404, "La planta no está en el catálogo.")
        _comprobar_error(c, e, r, "NO_ENCONTRADO", "GET /plantas/PL-999")
        s.pausa()

    if c.toca(7):
        _bd_caida(c, "HU-01 · Criterio 7", 7,
                  "Dado que PostgreSQL no está disponible, cuando se consulta GET /plantas o "
                  "GET /plantas/{plantaId}, entonces responde 503 NO_DISPONIBLE.",
                  [(rep, "/plantas"), (pro, "/plantas/PL-001")])  # fmt: skip
        s.pausa()


# ---------------------------------------------------------------------------------------------
# HU-INV-02 · Consultar activos
# ---------------------------------------------------------------------------------------------
def hu_02(c: Contexto) -> None:
    s = c.salida
    s.titulo("HU-INV-02 · Consultar el catálogo de activos")
    s.nota("Como Telemetría, Reportes, Pronóstico o Mantenimiento requiero listar y consultar los")
    s.nota("activos físicos con su estado vigente, filtrando por planta, tipo y estado.")
    if not c.entorno.disponible:
        s.nota(f"Sin el docker compose local se omite la evidencia en BD ({c.entorno.motivo}).")
    rep, man, tel = c.reportes, c.mantenimiento, c.telemetria

    if c.toca(1):
        e = "HU-02 · Criterio 1"
        s.criterio(1, "Dado que el catálogo tiene activos, cuando se envía GET /activos sin parámetros, "
                   "entonces responde 200 con como máximo 50 activos ordenados por id y total = 1270.")  # fmt: skip
        r, ok = c.http(rep, "GET", "/activos", 200, "Primera página del catálogo de activos.")
        s.comprobar(e, "responde 200", ok)
        _pagina_por_defecto(c, e, r, TOTAL_ACTIVOS)
        c.evidencia_sql(e, "el total de activos", "select count(*) from activos", str(TOTAL_ACTIVOS))
        s.pausa()

    if c.toca(2):
        e = "HU-02 · Criterio 2"
        s.criterio(2, "Dado que se filtra por planta, cuando se envía GET /activos?plantaId=PL-001, "
                   "entonces solo devuelve activos de PL-001 (22) y todos los id empiezan por PL-001-.")  # fmt: skip
        r, ok = c.http(rep, "GET", "/activos?plantaId=PL-001", 200, "Los activos de la planta PL-001.")  # fmt: skip
        datos, total = _datos(r), _cuerpo(r).get("total")
        s.comprobar(e, f"total {total} y {len(datos)} activos (se esperaban 22)", ok and total == 22 and len(datos) == 22)  # fmt: skip
        s.comprobar(e, "todos los id empiezan por PL-001- y su plantaId es PL-001", bool(datos) and all(str(a.get("id")).startswith("PL-001-") and a.get("plantaId") == "PL-001" for a in datos))  # fmt: skip
        c.evidencia_sql(e, "los activos de PL-001", "select count(*) from activos where planta_id = 'PL-001'", "22")  # fmt: skip
        s.pausa()

    if c.toca(3):
        e = "HU-02 · Criterio 3"
        s.criterio(3, "Dado que se envían plantaId, tipo y estado, cuando se combinan, entonces se "
                   "aplican con AND (PL-002 & turbina & en_mantenimiento → 1; estado=en_mantenimiento → 2).")  # fmt: skip
        casos = [
            ({"plantaId": "PL-002", "tipo": "turbina", "estado": "en_mantenimiento"}, 1),
            ({"estado": "en_mantenimiento"}, 2),
            ({"plantaId": "PL-002", "tipo": "turbina"}, 4),
            ({"plantaId": "PL-002", "estado": "en_mantenimiento", "tipo": "banco_baterias"}, 0),
        ]
        for filtros, esperado in casos:
            consulta = "&".join(f"{k}={v}" for k, v in filtros.items())
            r, ok = c.http(man, "GET", f"/activos?{consulta}", 200, f"Activos que cumplen todos los filtros a la vez ({consulta}).")  # fmt: skip
            total, datos = _cuerpo(r).get("total"), _datos(r)
            cumplen = all(a.get(k) == v for a in datos for k, v in filtros.items())
            s.comprobar(e, f"{consulta}: total {total} (se esperaba {esperado}) y cada activo cumple todos los filtros", ok and total == esperado == len(datos) and cumplen)  # fmt: skip
        c.evidencia_sql(e, "los activos en mantenimiento", "select string_agg(id, ',' order by id) from activos where estado = 'en_mantenimiento'", "PL-002-TB-01,PL-003-PS-01")  # fmt: skip
        s.pausa()

    if c.toca(4):
        e = "HU-02 · Criterio 4"
        s.criterio(4, "Dado que se envían limit (1 a 200) y offset (≥ 0), cuando la API responde, "
                   "entonces aplica las mismas reglas que HU-INV-01.")  # fmt: skip
        _paginacion(c, e, rep, "/activos", TOTAL_ACTIVOS)
        s.pausa()

    if c.toca(5):
        e = "HU-02 · Criterio 5"
        s.criterio(5, "Dado que un filtro es inválido (tipo=reactor, estado=roto, limit=500, plantaId "
                   "fuera de patrón), cuando la API lo valida, entonces responde 400 SOLICITUD_INVALIDA.")  # fmt: skip
        _invalidos(c, e, man, [
            ("/activos?tipo=reactor", "tipo fuera del catálogo (panel_solar, turbina, banco_baterias)"),
            ("/activos?estado=roto", "estado fuera del catálogo"),
            ("/activos?limit=500", "limit mayor que 200"),
            ("/activos?plantaId=PL.001", "plantaId con un carácter fuera de ^[A-Za-z0-9_-]+$"),
        ])  # fmt: skip
        s.pausa()

    if c.toca(6):
        e = "HU-02 · Criterio 6"
        s.criterio(6, "Dado que el activo existe, cuando se envía GET /activos/PL-001-PS-07, entonces "
                   "responde 200 con plantaId = PL-001.")  # fmt: skip
        r, ok = c.http(tel, "GET", "/activos/PL-001-PS-07", 200, "El activo con su estado vigente.")
        cuerpo = _cuerpo(r)
        s.comprobar(e, f"id {cuerpo.get('id')}, plantaId {cuerpo.get('plantaId')}, tipo {cuerpo.get('tipo')}, estado {cuerpo.get('estado')}", ok and cuerpo.get("id") == "PL-001-PS-07" and cuerpo.get("plantaId") == "PL-001")  # fmt: skip
        c.evidencia_sql(e, "la planta de PL-001-PS-07", "select planta_id from activos where id = 'PL-001-PS-07'", "PL-001")  # fmt: skip
        s.pausa()

    if c.toca(7):
        e = "HU-02 · Criterio 7"
        s.criterio(7, "Dado que el activo no existe, cuando se envía GET /activos/PL-001-PS-99, "
                   "entonces responde 404 NO_ENCONTRADO.")  # fmt: skip
        r, _ = c.http(tel, "GET", "/activos/PL-001-PS-99", 404, "El activo no está en el catálogo.")
        _comprobar_error(c, e, r, "NO_ENCONTRADO", "GET /activos/PL-001-PS-99")
        s.pausa()

    if c.toca(8):
        _bd_caida(c, "HU-02 · Criterio 8", 8,
                  "Dado que PostgreSQL no está disponible, cuando se consulta GET /activos o "
                  "GET /activos/{id}, entonces responde 503 NO_DISPONIBLE.",
                  [(rep, "/activos"), (tel, "/activos/PL-001-PS-07")])  # fmt: skip
        s.pausa()


# ---------------------------------------------------------------------------------------------
# Registro de historias: para agregar una, escriba hu_NN(c) y súmela aquí.
HISTORIAS: dict[str, Callable[[Contexto], None]] = {
    "01": hu_01,
    "02": hu_02,
}
PENDIENTES = ("03", "04", "05", "06", "07", "08")


def _parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="python -m scripts.demo.criterios",
        description="Demo por criterios de aceptación de una historia de usuario de Inventario.",
        parents=[crear_parser(add_help=False)],
    )
    p.add_argument("--hu", required=True, help=f"Historia: {', '.join(HISTORIAS)} o todas")
    p.add_argument("--solo", default="", help="Solo estos criterios, separados por comas (ej. 3,4)")
    p.add_argument("--sin-parar-bd", action="store_true", help="No detener PostgreSQL (criterio del 503)")  # fmt: skip
    return p


def main(
    argv: Sequence[str] | None = None,
    http: httpx.Client | None = None,
    entorno: Entorno | None = None,
    escribir: Callable[[str], None] = print,
    leer: Callable[[str], str] = input,
) -> int:
    args = _parser().parse_args(sys.argv[1:] if argv is None else argv)
    config = configuracion_de(args)
    salida = Salida(
        color=config.color and sys.stdout.isatty(),
        pausar=not config.rapido and sys.stdin.isatty(),
        escribir=escribir,
        leer=leer,
    )
    hu = args.hu.strip().lower()
    if hu.isdigit():
        hu = hu.zfill(2)
    if hu in PENDIENTES:
        salida.linea(f"✗ HU-INV-{hu} aún no implementada en la demo (hoy: {', '.join(HISTORIAS)}).")
        return SIN_EJECUTAR
    if hu != "todas" and hu not in HISTORIAS:
        salida.linea(f"✗ Historia desconocida: {args.hu}. Use {', '.join(HISTORIAS)} o todas.")
        return SIN_EJECUTAR
    if not re.fullmatch(r"[\d,\s]*", args.solo):
        salida.linea(f"✗ --solo espera números separados por comas (recibió {args.solo!r}).")
        return SIN_EJECUTAR
    solo = {int(x) for x in args.solo.split(",") if x.strip().isdigit()}
    cliente = http or httpx.Client(base_url=config.url, timeout=20)
    api = ClienteApi(cliente)
    contexto = Contexto(
        config, salida, api,
        entorno or DockerLocal(Path.cwd(), config.compose_proyecto, config.compose_archivos, config.url),
        solo, Telemetria(api, salida), Reportes(api, salida), Pronostico(api, salida),
        Mantenimiento(api, salida), parar_bd=not args.sin_parar_bd,
    )  # fmt: skip
    seleccion = list(HISTORIAS) if hu == "todas" else [hu]
    try:
        salida.linea("═" * 68)
        salida.linea(f"  Inventario de Activos · Demo por criterios · API: {config.url}")
        salida.linea("═" * 68)
        salida.comando(f"export DEMO_URL={config.url}", "Preparación para copiar los curl de ejemplo")
        if contexto.entorno.disponible:
            salida.nota(f"docker compose local: {contexto.entorno.texto('ps')}")
        else:
            salida.nota(f"docker compose local no disponible: {contexto.entorno.motivo}")
        r = api.enviar("GET", "/health")
        if r.estado != 200:
            salida.linea(f"✗ La API no responde como se espera (GET /health → {r.estado} {r.cuerpo})")
            return SIN_EJECUTAR
        for clave in seleccion:
            salida.linea()
            HISTORIAS[clave](contexto)
    except httpx.TransportError:
        salida.linea(f"✗ No se pudo conectar a {config.url}. ¿Está levantada la API? "
                     "(docker compose up -d --build --wait)")  # fmt: skip
        return SIN_EJECUTAR
    finally:
        if http is None:
            cliente.close()
    return CORRECTO if salida.resumen() else COMPROBACION_FALLIDA


if __name__ == "__main__":
    raise SystemExit(main())
