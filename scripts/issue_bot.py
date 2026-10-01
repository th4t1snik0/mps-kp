#!/usr/bin/env python3
"""Бот Issue «Мой курсач»: студент (или его нейронка) работает с курсачом через Issue, без прав на запись в репо.

Вызывает .github/workflows/issue.yml. Вход — переменные окружения GitHub (событие, номер Issue, автор, права автора)
и тексты Issue/комментария в файлах ISSUE_BODY_FILE / COMMENT_FILE (текст пользователя — только данные, в shell не попадает).
Делает: пишет файлы ТОЛЬКО в students/<ник>/ этого Issue, печатает в GITHUB_OUTPUT:
  nick — ник студента; dispatch — какие джобы запустить (km1 km1-next km2 km3); commit — 1, если файлы менялись;
  reply — путь к файлу с ответом в Issue.

Форма Issue (.github/ISSUE_TEMPLATE/kursach.yml): ФИО, группа, вариант, преподаватель, стиль.
Команды в комментариях (строка с «/» в начале, блок ``` после неё — содержимое файла):
  /помощь
  /заготовки                     — заготовки программ, ПЗ1, ПЗ2 и схем алгоритмов (make code-init, pz1-init, pz2-init)
  /схема  (/км1)                 — пересобрать схему (КМ-1)
  /всё                           — КМ-1, затем КМ-2 и КМ-3 по новой схеме
  /пз1  (/км2)  [+ ```md```]     — записать students/<ник>/pz/pz1.md (если есть блок) и собрать ПЗ1
  /пз2  [+ ```md```]             — записать pz/pz2.md и собрать ПЗ2 (КМ-3)
  /prog1 | /prog2 | /prog3 + ``` — записать code/progN.a51 и проверить (КМ-3)
  /flow <имя> + ```              — записать pz/flow/<имя>.flow (КМ-3)
  /правка + ```yaml```           — записать schema/fixes.yaml (правки схемы по замечаниям), затем как /всё
  /замечание КМ-N <текст>        — записать замечание руководителя в remarks.md (дословно)
"""
import os
import re
import subprocess
import sys
import time

ROOT = os.getcwd()
MAX_FILE = 200_000  # байт на файл из комментария

HELP = """**Команды** (пишите комментарием; содержимое файла — блоком ``` сразу после команды):

| Команда | Что делает |
| --- | --- |
| `/заготовки` | кладёт заготовки программ (prog1–3), ПЗ1, ПЗ2 и схем алгоритмов в папку студента — дальше их можно править |
| `/схема` | пересобрать схему (КМ-1) |
| `/всё` | КМ-1, затем ПЗ1 (КМ-2) и программы + ПЗ2 (КМ-3) по новой схеме |
| `/пз1` + блок текста | КМ-2: записать `pz/pz1.md` (места «ДОПИШИ») и собрать ПЗ1; без блока — просто собрать |
| `/prog1` + блок кода | КМ-3: записать `code/prog1.a51` и проверить (то же `/prog2`, `/prog3`) |
| `/пз2` + блок текста | записать `pz/pz2.md` и собрать ПЗ2 |
| `/flow 02-main` + блок | записать схему алгоритма `pz/flow/02-main.flow` |
| `/правка` + блок yaml | правки схемы по замечаниям → `schema/fixes.yaml` (номиналы, перечень, «Примечание», сдвиги), затем `/всё` |
| `/замечание КМ-1 текст` | записать замечание руководителя в историю (`remarks.md`), дословно |
| `/файл pz/flow/14-x.flow` + блок | записать любой файл в папке студента (`.a51`, `.md`, `.flow`, `.yaml`) |
| `/удалить pz/flow/14-x.flow` | удалить файл из папки студента (кроме `variant.yaml`, `issue`, `remarks.md`) |
| `/доступ` | попросить у владельца репо права на запись (работать пушами, Run workflow) |

Файлы студента лежат в репо: `students/{nick}/` — их можно читать (нейронке — тоже) и присылать исправленные целиком.
Как писать программы и ПЗ — `docs/code-guide.md`, `docs/pz-guide.md`, формат правок — `AGENTS.md`, «Замечания руководителя».
"""


def out(key, val):
    with open(os.environ.get("GITHUB_OUTPUT", "/dev/stdout"), "a") as f:
        f.write(f"{key}={val}\n")


def read(path):
    try:
        with open(path, encoding="utf-8") as f:
            return f.read()
    except OSError:
        return ""


def form_fields(body):
    """Поля формы Issue: «### Заголовок» → значение."""
    fields, cur = {}, None
    for line in body.splitlines():
        m = re.match(r"^###\s+(.+?)\s*$", line)
        if m:
            cur = m.group(1)
            fields[cur] = ""
        elif cur is not None:
            fields[cur] += line + "\n"
    return {k: ("" if v.strip() == "_No response_" else v.strip()) for k, v in fields.items()}


def pick(fields, *names):
    for k, v in fields.items():
        if any(n.lower() in k.lower() for n in names):
            return v
    return ""


def run(*cmd):
    r = subprocess.run(cmd, capture_output=True, text=True)
    return r.returncode, (r.stdout + r.stderr).strip()


def nick_of(group, fio):
    code, o = run("bin/mpsgen", "-nick", "-group", group, "-name", fio)
    return o if code == 0 else ""


def blocks_after(text):
    """Команды и блоки: [(команда, аргументы, содержимое блока или None)]."""
    lines = text.splitlines()
    res, i = [], 0
    while i < len(lines):
        m = re.match(r"^\s*/(\S+)\s*(.*)$", lines[i])
        if not m:
            i += 1
            continue
        cmd, args, block = m.group(1).lower(), m.group(2).strip(), None
        j = i + 1
        while j < len(lines) and not lines[j].strip():
            j += 1
        if j < len(lines) and lines[j].lstrip().startswith("```"):
            fence = re.match(r"^\s*(`{3,})", lines[j]).group(1)
            k, body = j + 1, []
            while k < len(lines) and not lines[k].strip().startswith(fence):
                body.append(lines[k])
                k += 1
            block = "\n".join(body) + "\n"
            i = k + 1
        else:
            i += 1
        res.append((cmd, args, block))
    return res


ALLOWED = re.compile(r"^(code/[\w.-]+\.a51|pz/[\w.-]+\.md|pz/flow/[\w.-]+\.flow|schema/[\w.-]+\.yaml|[\w.-]+\.md)$")


def safe_rel(rel):
    """Путь внутри папки студента: только известные места и расширения, без «..»."""
    rel = rel.strip().strip("`").lstrip("./")
    if ".." in rel.split("/") or not ALLOWED.match(rel):
        raise ValueError(f"путь «{rel}» нельзя: можно code/*.a51, pz/*.md, pz/flow/*.flow, schema/*.yaml, *.md")
    return rel


def km_for(rel):
    """Какую джобу пересобрать после правки файла."""
    if rel.startswith("schema/"):
        return "km1-next"
    if rel == "pz/pz1.md":
        return "km2"
    if rel.startswith("code/") or rel.startswith("pz/"):
        return "km3"
    return ""


def write(nick, rel, content, changed):
    if len(content.encode()) > MAX_FILE:
        raise ValueError(f"`{rel}` больше {MAX_FILE // 1000} КБ")
    path = os.path.join(ROOT, "students", nick, rel)
    if os.path.commonpath([os.path.realpath(path), os.path.realpath(os.path.join(ROOT, "students", nick))]) != os.path.realpath(os.path.join(ROOT, "students", nick)):
        raise ValueError("путь вне папки студента")
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)
    changed.append(f"students/{nick}/{rel}")


def variant_yaml(f):
    lines = [f"group: {f['group']}", f"m: {f['m']}", f"name: {f['fio']}"]
    if f.get("checker"):
        lines.append(f"checker: {f['checker']}")
    if f.get("style") and f["style"] not in ("авто", "auto"):
        lines.append(f"style: {f['style']}")
    return "# создано из Issue #{} — правки делать через Issue (поменять форму или команды в комментариях)\n".format(
        os.environ.get("ISSUE_NUMBER", "?")) + "\n".join(lines) + "\n"


def main():
    event = os.environ.get("EVENT", "")
    issue = os.environ.get("ISSUE_NUMBER", "")
    issue_body = read(os.environ.get("ISSUE_BODY_FILE", ""))
    reply, changed, dispatch = [], [], []
    f = form_fields(issue_body)
    fio = re.sub(r"\s+", " ", pick(f, "ФИО", "Фамилия")).strip()
    group = pick(f, "Группа").split()[0] if pick(f, "Группа") else ""
    m = pick(f, "Вариант")
    form = {"fio": fio, "group": group, "m": m, "checker": pick(f, "Преподаватель", "Проверяющий"), "style": pick(f, "Стиль")}
    errs = []
    if not re.fullmatch(r"[А-ЯЁ][а-яё\-]+ [А-ЯЁ]\.\s?[А-ЯЁ]?\.?", fio):
        errs.append(f"ФИО «{fio}» — нужно «Фамилия И.О.», например «Рязанцев И.В.»")
    if not re.fullmatch(r"\d{1,2}", m) or not 1 <= int(m) <= 30:
        errs.append(f"вариант «{m}» — число 1…30")
    nick = nick_of(group, fio) if not errs else ""
    if not errs and not nick:
        errs.append("не получилось посчитать ник по группе и ФИО")
    if errs:
        reply.append("❌ Не могу принять форму:\n" + "\n".join(f"- {e}" for e in errs) +
                     "\n\nИсправьте описание Issue (… → Edit) — бот перечитает его.")
        finish("", [], [], reply)
        return
    sdir = os.path.join(ROOT, "students", nick)
    link = os.path.join(sdir, "issue")
    own = read(link).strip()
    if own and own != issue:
        reply.append(f"❌ Папка `students/{nick}/` уже ведётся в Issue #{own}. Работайте там (или закройте старый Issue и напишите руководителю репо).")
        finish("", [], [], reply)
        return

    if event in ("opened", "edited", "reopened"):
        new = variant_yaml(form)
        old = read(os.path.join(sdir, "variant.yaml"))
        strip = lambda s: "\n".join(l for l in s.splitlines() if not l.startswith("#"))
        if strip(new) != strip(old):
            write(nick, "variant.yaml", new, changed)
            dispatch.append("km1-next")
        if own != issue:
            write(nick, "issue", issue + "\n", changed)
        if event == "opened" or not old:
            reply.append(f"Принято: **{fio}**, {group}, вариант {m}. Ник (папка): `students/{nick}/`.\n\n"
                         "Сейчас соберётся схема (КМ-1) — результат придёт сюда комментарием, со ссылкой на Яндекс-диск.\n\n" +
                         HELP.format(nick=nick))
        elif changed:
            reply.append("Данные варианта обновлены — пересобираю схему, затем ПЗ1 и ПЗ2 по ней.")
        finish(nick, changed, dispatch, reply)
        return

    # комментарий
    text = read(os.environ.get("COMMENT_FILE", ""))
    cmds = blocks_after(text)
    if not cmds:
        return finish(nick, [], [], [])  # обычное обсуждение — молчим
    if not os.path.exists(os.path.join(sdir, "variant.yaml")):
        write(nick, "variant.yaml", variant_yaml(form), changed)
        write(nick, "issue", issue + "\n", changed)
        dispatch.append("km1")
    for cmd, args, block in cmds:
        try:
            if cmd in ("помощь", "help"):
                reply.append(HELP.format(nick=nick))
            elif cmd in ("заготовки", "init"):
                done = []
                for target in ("code-init", "pz2-init"):
                    code, o = run("make", target, f"S={nick}")
                    if code != 0:
                        raise ValueError(f"`make {target}`:\n```\n{o[-1500:]}\n```")
                done += ["code/", "pz/pz2.md", "pz/flow/"]
                # заготовка ПЗ1 строится по схеме — нужна готовая СХЕМА-N
                if run("scripts/schema_fetch.sh", nick)[0] == 0 and run("make", "pz1-init", f"S={nick}")[0] == 0:
                    done.append("pz/pz1.md")
                else:
                    reply.append("Заготовку ПЗ1 положу, когда будет схема: дождитесь комментария со схемой и пришлите `/заготовки` ещё раз.")
                changed.append(f"students/{nick}/")
                repo = os.environ.get("GITHUB_REPOSITORY", "")
                raw = f"https://raw.githubusercontent.com/{repo}/main/students/{nick}"
                links = [f"[code/prog{n}.a51]({raw}/code/prog{n}.a51)" for n in (1, 2, 3)] + \
                        [f"[pz/{f}]({raw}/pz/{f})" for f in ("pz1.md", "pz2.md") if f"pz/{f}" in done or f == "pz2.md"]
                reply.append(f"Заготовки положены в `students/{nick}/`: " + ", ".join(done) + ".\n\n"
                             "**Для нейронки** — прочитать файлы по прямым ссылкам (появятся через минуту, после коммита бота): " +
                             " · ".join(links) + f" · схемы алгоритмов — `students/{nick}/pz/flow/`.\n"
                             f"Как писать программы — https://github.com/{repo}/blob/main/docs/code-guide.md (раздел 0), "
                             f"ПЗ — https://github.com/{repo}/blob/main/docs/pz-guide.md.\n\n"
                             "Готовое присылайте **файлом целиком**: комментарий `/prog1` и сразу под ним блок ```asm … ``` "
                             "(так же `/prog2`, `/prog3`, `/пз1`, `/пз2`, `/flow <имя>`) — бот запишет и проверит.")
            elif cmd in ("схема", "км1", "km1"):
                dispatch.append("km1")
            elif cmd in ("всё", "все", "all"):
                dispatch.append("km1-next")
            elif cmd in ("пз1", "км2", "km2", "pz1"):
                if block is not None:
                    write(nick, "pz/pz1.md", block, changed)
                dispatch.append("km2")
            elif cmd in ("пз2", "pz2", "км3", "km3"):
                if block is not None:
                    write(nick, "pz/pz2.md", block, changed)
                dispatch.append("km3")
            elif re.fullmatch(r"prog[123]", cmd):
                if block is None:
                    raise ValueError(f"/{cmd}: нужен блок с кодом (```…```) сразу после команды")
                write(nick, f"code/{cmd}.a51", block, changed)
                dispatch.append("km3")
            elif cmd == "flow":
                name = re.sub(r"\.flow$", "", args)
                if not re.fullmatch(r"[0-9A-Za-z_-]{1,40}", name) or block is None:
                    raise ValueError("/flow <имя> + блок: имя латиницей/цифрами (например `02-main`), содержимое — блоком")
                write(nick, f"pz/flow/{name}.flow", block, changed)
                dispatch.append("km3")
            elif cmd in ("правка", "fix", "fixes"):
                if block is None:
                    raise ValueError("/правка: нужен блок ```yaml с правками (формат — AGENTS.md, «Замечания руководителя»)")
                write(nick, "schema/fixes.yaml", block, changed)
                dispatch.append("km1-next")
            elif cmd in ("файл", "file"):
                rel = safe_rel(args)
                if block is None:
                    raise ValueError(f"/файл {args}: нужен блок с содержимым (```…```) сразу после команды")
                write(nick, rel, block, changed)
                dispatch.append(km_for(rel))
            elif cmd in ("удалить", "delete", "rm"):
                rel = safe_rel(args)
                if rel in ("variant.yaml", "issue", "remarks.md"):
                    raise ValueError(f"`{rel}` не удаляется (данные варианта — правкой формы Issue, история замечаний — только дописывается)")
                path = os.path.join(sdir, rel)
                if not os.path.isfile(path):
                    raise ValueError(f"нет файла `students/{nick}/{rel}`")
                os.remove(path)
                changed.append(f"students/{nick}/{rel}")
                dispatch.append(km_for(rel))
                reply.append(f"Удалён `students/{nick}/{rel}`.")
            elif cmd in ("доступ", "access"):
                who = os.environ.get("COMMENT_AUTHOR", "")
                owner = os.environ.get("REPO_OWNER", "")
                repo = os.environ.get("GITHUB_REPOSITORY", "")
                reply.append(f"@{owner}: **@{who}** просит права на запись в репо (чтобы работать пушами и Run workflow).\n\n"
                             f"Добавить: Settings → Collaborators → Add people → `{who}` (роль Write), или одной командой:\n"
                             f"```sh\ngh api -X PUT repos/{repo}/collaborators/{who} -f permission=push\n```\n"
                             "Пока доступа нет, всё работает и через команды в этом Issue.")
            elif cmd in ("замечание", "remark"):
                mm = re.match(r"(?:КМ-?|KM-?)?(\d)\s+(.*)", args, re.S | re.I)
                body = (mm.group(2) if mm else args) + ("\n" + block if block else "")
                if not body.strip():
                    raise ValueError("/замечание КМ-1 текст замечания")
                code, o = run("bin/mpsgen", "-student", f"students/{nick}/variant.yaml", "-remark", body.strip(),
                              "-km", mm.group(1) if mm else "", "-by", "")
                if code != 0:
                    raise ValueError(o)
                changed.append(f"students/{nick}/remarks.md")
                reply.append("Замечание записано в `remarks.md`. Когда исправите — допишите туда «Что сделали» (или пришлите правку `/правка`).")
            else:
                reply.append(f"Не знаю команду `/{cmd}` — `/помощь`.")
        except ValueError as e:
            reply.append(f"❌ {e}")
    finish(nick, changed, dispatch, reply)


def finish(nick, changed, dispatch, reply):
    out("nick", nick)
    out("commit", "1" if changed else "")
    # km1-next включает km2 и km3 (их запустит КМ-1 по готовности схемы)
    d = []
    dispatch = [k for k in dispatch if k]
    for k in ("km1-next", "km1", "km2", "km3"):
        if k in dispatch and k not in d:
            d.append(k)
    if "km1-next" in d:
        d = ["km1-next"]
    elif "km1" in d:
        d = [k for k in d if k != "km2"]  # ПЗ1 по старой схеме не нужна — после /схема её можно запросить /пз1
    if d:
        names = {"km1-next": "схема → ПЗ1 и ПЗ2", "km1": "схема (КМ-1)", "km2": "ПЗ1 (КМ-2)", "km3": "программы и ПЗ2 (КМ-3)"}
        reply.append("⏳ Запущено: " + ", ".join(names[k] for k in d) + ". Результат придёт сюда.")
    out("dispatch", " ".join(d))
    path = os.path.join(os.environ.get("RUNNER_TEMP", "/tmp"), f"reply-{int(time.time())}.md")
    with open(path, "w", encoding="utf-8") as f:
        f.write("\n\n".join(reply))
    out("reply", path if reply else "")


if __name__ == "__main__":
    sys.exit(main())
