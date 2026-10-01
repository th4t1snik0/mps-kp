#!/usr/bin/env python3
"""Загрузка результатов сборки на Яндекс-диск: <папка по ссылке>/<группа>/<M> <ФИО>/<ВИД>-N[ по СХЕМА-K]/<файлы>.

    YADISK_TOKEN=… YADISK_PUBLIC=https://disk.yandex.ru/d/… scripts/yadisk_upload.py build/А-12-23/14 …

Что и куда — готовит publish.sh: build/<группа>/<вариант>/.pub (содержимое прогона), .pubdest (папка студента)
и .run (имя папки прогона — как в ветке results). Номер на диске занят — берётся следующий номер того же вида.
Папку назначения задаём публичной ссылкой (YADISK_PUBLIC): скрипт сам находит её путь на диске
владельца токена. Недостающие папки создаются, существующие файлы перезаписываются.
Токен: OAuth-приложение с правами «Чтение всего Диска» + «Запись в любом месте на Диске» (см. README).
"""
import json
import os
import re
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
            run = open(os.path.join(d, ".run")).read().strip()        # «ПЗ1-2 по СХЕМА-3»
            dest = open(os.path.join(d, ".pubdest")).read().strip()   # «А-12-23/17 Рязанцев И.В.» (или «_пробы/…»)
        except OSError:
            run = dest = ""
        m = re.match(r"^(\S+?)-(\d+)(.*)$", run)
        if not os.path.isdir(pub) or not dest or not m:
            print(f"яндекс-диск: {d} — нет .pub/.run/.pubdest (publish.sh не отработал?), пропуск")
            continue
        kind, n, rest = m.group(1), int(m.group(2)), m.group(3)
        cur = base
        for part in dest.split("/"):  # группа, «M ФИО» — создаём недостающие
            cur = f"{cur}/{part}"
            mkdir(cur)
        if exists(f"{cur}/{kind}-{n}{rest}") or last_run(cur, kind) >= n:  # номер занят (диск чистили руками) — следующий
            n = max(n, last_run(cur, kind) + 1)
        dst = f"{cur}/{kind}-{n}{rest}"
        print(f"яндекс-диск: {dst} — {put_tree(pub, dst)} файлов")
        with open(os.path.join(d, ".disk"), "w", encoding="utf-8") as f:  # для комментария в Issue (issue_notify.py)
            f.write(f"{dest}/{kind}-{n}{rest}\n")


def exists(path):
    code, _ = call("GET", "/resources", path=path, fields="name")
    return code == 200


def last_run(path, kind):
    """Наибольший N среди папок «<kind>-N…» в path (0 — нет)."""
    code, d = call("GET", "/resources", path=path, limit=1000, fields="_embedded.items.name")
    best = 0
    for it in (d.get("_embedded", {}).get("items", []) if code == 200 else []):
        m = re.match(r"^" + re.escape(kind) + r"-(\d+)( |$)", it.get("name", ""))
        if m:
            best = max(best, int(m.group(1)))
    return best


if __name__ == "__main__":
    main(sys.argv[1:])
