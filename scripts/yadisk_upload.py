#!/usr/bin/env python3
"""Загрузка результатов сборки на Яндекс-диск: <папка по ссылке>/<группа>/<вариант>/<ВИД>-N/<файлы>.

    YADISK_TOKEN=… YADISK_PUBLIC=https://disk.yandex.ru/d/… scripts/yadisk_upload.py build/А-12-23/14 …

Что и куда — готовит publish.sh: build/<группа>/<вариант>/.pub (содержимое прогона) и .run (имя папки: СХЕМА-N,
ПЗ1-N или ПЗ2-N — как в ветке results). Если такая папка на диске уже есть — берётся следующий номер того же вида.
Папку назначения задаём публичной ссылкой (YADISK_PUBLIC): скрипт сам находит её путь на диске
владельца токена. Недостающие папки создаются, существующие файлы перезаписываются.
Токен: OAuth-приложение с правами «Чтение всего Диска» + «Запись в любом месте на Диске» (см. README).
"""
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

API = "https://cloud-api.yandex.net/v1/disk"
TOKEN = os.environ.get("YADISK_TOKEN", "")
PUBLIC = os.environ.get("YADISK_PUBLIC", "")


def call(method, endpoint, **params):
    url = API + endpoint + ("?" + urllib.parse.urlencode(params) if params else "")
    for attempt in range(12):
        req = urllib.request.Request(url, method=method, headers={"Authorization": "OAuth " + TOKEN})
        try:
            with urllib.request.urlopen(req) as r:
                body = r.read()
                return r.status, json.loads(body) if body else {}
        except urllib.error.HTTPError as e:
            body = e.read()
            # 423 — ресурс занят (параллельный запуск пишет в ту же папку), 429/5xx — подождать и повторить
            # 423 — папку в этот момент меняет другая джоба (КМ-1/2/3 публикуют одновременно), 429/5xx — подождать и повторить
            if e.code in (423, 429, 500, 502, 503) and attempt < 11:
                time.sleep(min(3 * (attempt + 1), 15))
                continue
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
    if code == 423 and exists(path):  # заблокирована соседней джобой, но уже создана
        return
    if code not in (201, 409):  # 409 — уже есть
        sys.exit(f"яндекс-диск: mkdir {path} — {code} {d.get('description', d)}")


def upload(local, remote):
    code, d = call("GET", "/resources/upload", path=remote, overwrite="true")
    if code != 200:
        sys.exit(f"яндекс-диск: upload {remote} — {code} {d.get('description', d)}")
    with open(local, "rb") as f:
        req = urllib.request.Request(d["href"], data=f.read(), method="PUT")
        urllib.request.urlopen(req).read()


def put_tree(local, remote):
    """Заливает папку local целиком (с подпапками) в remote; возвращает число файлов."""
    mkdir(remote)
    n = 0
    for f in sorted(os.listdir(local)):
        p = os.path.join(local, f)
        if os.path.isdir(p):
            n += put_tree(p, f"{remote}/{f}")
        elif os.path.isfile(p):
            upload(p, f"{remote}/{f}")
            n += 1
    return n


def main(dirs):
    if not TOKEN or not PUBLIC:
        print("яндекс-диск: нет YADISK_TOKEN/YADISK_PUBLIC — пропуск")
        return
    base = base_path()
    for d in dirs:
        d = d.rstrip("/")
        pub = os.path.join(d, ".pub")
        try:
            run = open(os.path.join(d, ".run")).read().strip()
        except OSError:
            run = ""
        kind, _, num = run.rpartition("-")
        if not os.path.isdir(pub) or not kind or not num.isdigit():
            print(f"яндекс-диск: {d} — нет .pub/.run (publish.sh не отработал?), пропуск")
            continue
        name = os.path.relpath(d, "build") if d.startswith("build/") else os.path.basename(d)
        cur = base
        for part in name.split("/"):  # группа, затем вариант — создаём недостающие
            cur = f"{cur}/{part}"
            mkdir(cur)
        n = int(num)
        if exists(f"{cur}/{kind}-{n}"):  # номер уже занят (диск чистили руками или ветку results) — следующий
            n = max(n, last_run(cur, kind) + 1)
        dst = f"{cur}/{kind}-{n}"
        print(f"яндекс-диск: {dst} — {put_tree(pub, dst)} файлов")


def exists(path):
    code, _ = call("GET", "/resources", path=path, fields="name")
    return code == 200


def last_run(path, kind):
    """Наибольший N среди папок «<kind>-N» в path (0 — нет)."""
    code, d = call("GET", "/resources", path=path, limit=1000, fields="_embedded.items.name")
    best, pre = 0, kind + "-"
    for it in (d.get("_embedded", {}).get("items", []) if code == 200 else []):
        name = it.get("name", "")
        if name.startswith(pre) and name[len(pre):].isdigit():
            best = max(best, int(name[len(pre):]))
    return best


if __name__ == "__main__":
    main(sys.argv[1:])
