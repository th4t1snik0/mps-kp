#!/usr/bin/env python3
"""Загрузка результатов сборки на Яндекс-диск: <папка по ссылке>/<вариант>/<файлы>.

    YADISK_TOKEN=… YADISK_PUBLIC=https://disk.yandex.ru/d/… scripts/yadisk_upload.py build/try-А-12-14 …

Папку назначения задаём публичной ссылкой (YADISK_PUBLIC): скрипт сам находит её путь на диске
владельца токена. Недостающие папки создаются, существующие файлы перезаписываются.
Токен: OAuth-приложение с правами «Чтение всего Диска» + «Запись в любом месте на Диске» (см. README).
"""
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

API = "https://cloud-api.yandex.net/v1/disk"
KEEP = ["params.md", "params.json", "vars.inc", "schematic.png", "schematic.pdf",
        "schematic.kicad_sch", "schematic.kicad_pro", "ramka.kicad_wks", "erc-summary.txt", "erc.rpt"]
TOKEN = os.environ.get("YADISK_TOKEN", "")
PUBLIC = os.environ.get("YADISK_PUBLIC", "")


def call(method, path, **params):
    url = API + path + ("?" + urllib.parse.urlencode(params) if params else "")
    req = urllib.request.Request(url, method=method, headers={"Authorization": "OAuth " + TOKEN})
    try:
        with urllib.request.urlopen(req) as r:
            body = r.read()
            return r.status, json.loads(body) if body else {}
    except urllib.error.HTTPError as e:
        body = e.read()
        return e.code, json.loads(body) if body else {}


def base_path():
    """Путь папки по публичной ссылке на диске владельца токена.

    Сравниваем по resource_id (публичная ссылка в списке опубликованного может быть в другом
    виде: yadi.sk/d/…, disk.yandex.ru/d/…); запасной вариант — по хвосту ссылки /d/<id>.
    """
    code, info = call("GET", "/public/resources", public_key=PUBLIC, fields="resource_id,name,type")
    if code != 200:
        sys.exit(f"яндекс-диск: ссылка {PUBLIC} — {code} {info.get('description', info)}")
    rid = info.get("resource_id", "")
    tail = PUBLIC.rstrip("/").rsplit("/d/", 1)[-1]
    offset, seen = 0, 0
    while True:
        code, d = call("GET", "/resources/public", limit=100, offset=offset, type="dir",
                       fields="items.path,items.public_url,items.resource_id")
        if code != 200:
            sys.exit(f"яндекс-диск: список опубликованного — {code} {d.get('description', d)} (токен и права?)")
        items = d.get("items", [])
        seen += len(items)
        for it in items:
            if (rid and it.get("resource_id") == rid) or it.get("public_url", "").rstrip("/").endswith("/d/" + tail):
                return it["path"]
        if len(items) < 100:
            # имена чужих папок в лог не пишем — лог публичного репо виден всем
            sys.exit(f"яндекс-диск: папка «{info.get('name')}» не найдена среди {seen} опубликованных папок владельца токена "
                     "(токен от другого аккаунта?)")
        offset += 100


def mkdir(path):
    code, d = call("PUT", "/resources", path=path)
    if code not in (201, 409):  # 409 — уже есть
        sys.exit(f"яндекс-диск: mkdir {path} — {code} {d.get('description', d)}")


def upload(local, remote):
    code, d = call("GET", "/resources/upload", path=remote, overwrite="true")
    if code != 200:
        sys.exit(f"яндекс-диск: upload {remote} — {code} {d.get('description', d)}")
    with open(local, "rb") as f:
        req = urllib.request.Request(d["href"], data=f.read(), method="PUT")
        urllib.request.urlopen(req).read()


def main(dirs):
    if not TOKEN or not PUBLIC:
        print("яндекс-диск: нет YADISK_TOKEN/YADISK_PUBLIC — пропуск")
        return
    base = base_path()
    for d in dirs:
        d = d.rstrip("/")
        if not os.path.isfile(os.path.join(d, "params.md")):
            continue
        name = os.path.basename(d)
        mkdir(f"{base}/{name}")
        n = 0
        for f in KEEP:
            p = os.path.join(d, f)
            if os.path.isfile(p):
                upload(p, f"{base}/{name}/{f}")
                n += 1
        print(f"яндекс-диск: {base}/{name} — {n} файлов")


if __name__ == "__main__":
    main(sys.argv[1:])
