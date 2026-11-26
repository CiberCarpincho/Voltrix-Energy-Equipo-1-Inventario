#!/usr/bin/env python3
"""Actualiza el proyecto Compose existente al commit publicado en main."""
import json
import os
from pathlib import Path
import pwd
import subprocess
import time
from urllib.error import URLError
from urllib.request import urlopen

ROOT = Path.cwd()
STATE = Path("/var/lib/voltrix-cd")
PENDING = STATE / "pendiente.json"
COMPOSE = ["docker", "compose", "--project-directory", str(ROOT / "deploy/compose"),
           "-f", str(ROOT / "deploy/compose/compose.yaml")]
BACKUP = "voltrix-inventario-api:rollback"


def run(*args):
    return subprocess.run(args, check=True, stdout=subprocess.PIPE,
                          text=True).stdout.strip()


def git(*args):
    owner = pwd.getpwuid(ROOT.stat().st_uid).pw_name
    return run("runuser", "-u", owner, "--", "git", *args)


def healthy():
    for attempt in range(20):
        try:
            with urlopen("http://127.0.0.1:8080/health", timeout=3) as response:
                body = json.load(response)
                if body.get("estado") == "disponible" and body.get("baseDatos") == "ok":
                    return
        except (OSError, URLError, ValueError):
            pass
        if attempt < 19:
            time.sleep(2)
    raise RuntimeError("La API no confirmó disponibilidad de PostgreSQL")


def start():
    subprocess.run([*COMPOSE, "up", "-d", "--no-build", "--wait",
                    "--wait-timeout", "120"], check=True)
    healthy()


def recover():
    previous = json.loads(PENDING.read_text())
    print("Restaurando", previous["commit"], flush=True)
    git("checkout", "--detach", previous["commit"])
    run("docker", "tag", BACKUP, previous["image"])
    start()
    PENDING.unlink()


def deploy():
    if PENDING.exists():
        recover()
    if git("status", "--porcelain", "--untracked-files=normal"):
        raise RuntimeError("Hay cambios locales en el clon de despliegue")
    git("fetch", "--no-tags", "origin", "refs/heads/main")
    target = git("rev-parse", "FETCH_HEAD")
    current = git("rev-parse", "HEAD")
    if target == current:
        return
    # Durante la instalación inicial main puede contener solo el esqueleto.
    if "infra/ec2/cd/desplegar.py" not in git("ls-tree", "--name-only", "-r", target).splitlines():
        print("Esperando la integración del despliegue continuo en main")
        return

    container = run(*COMPOSE, "ps", "-q", "api")
    if not container:
        raise RuntimeError("Se requiere el despliegue inicial de HU-04 en ejecución")
    old_id, old_name = run("docker", "inspect", "--format",
                           "{{.Image}} {{.Config.Image}}", container).split()
    run("docker", "tag", old_id, BACKUP)
    temporary = PENDING.with_suffix(".tmp")
    temporary.write_text(json.dumps({"commit": current, "image": old_name}))
    os.replace(temporary, PENDING)
    print("Desplegando", target, flush=True)
    try:
        git("checkout", "--detach", target)
        run(*COMPOSE, "config", "--quiet")
        subprocess.run([*COMPOSE, "build", "api"], check=True)
        start()
    except Exception:
        recover()
        raise
    PENDING.unlink()
    print("Versión saludable:", target, flush=True)


if __name__ == "__main__":
    deploy()
