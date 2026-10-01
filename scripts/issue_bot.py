#!/usr/bin/env python3
"""Бот Issue «Мой курсач» — курсач по шагам: студент (или его нейронка) работает через Issue, без прав на запись в репо.

Вызывает .github/workflows/issue.yml. Вход — переменные окружения GitHub (событие, номер Issue, автор) и тексты Issue/комментария
в файлах ISSUE_BODY_FILE / COMMENT_FILE (текст пользователя — только данные, в shell не попадает).
Пишет файлы ТОЛЬКО в students/<ник>/ этого Issue; в GITHUB_OUTPUT: nick, dispatch (km1 km1-next km2 km3), commit, reply.

Шаги (одна команда на шаг; файлы — блоками ``` с именем файла в первой строке блока):
  0. Issue по форме (ФИО, группа, вариант, преподаватель, стиль) → папка студента, схема (КМ-1).
  1. /км1                                  — пересобрать схему (КМ-1).
  2. /км2                                  — заготовка ПЗ1 + сборка; /км2 + блок ```pz1.md``` — записать текст и собрать (КМ-2).
  3. /км3                                  — заготовки программ, ПЗ2, схем алгоритмов; /км3 + блоки ```prog1.a51```, ```pz2.md```,
                                             ```02-main.flow``` — записать, проверить в эмуляторе, собрать ПЗ2 (КМ-3).
  Замечания: /замечание КМ-N <дословно>, /правка + ```fixes.yaml``` (правки схемы). /помощь — шаги и команды.
Старые команды (/заготовки, /prog1, /пз1, /пз2, /flow, /файл, /удалить, /схема, /всё, /доступ) работают как раньше.
"""
import glob
import os
import re
import subprocess
import sys
import time

ROOT = os.getcwd()
REPO = os.environ.get("GITHUB_REPOSITORY", "")
MAX_FILE = 200_000  # байт на файл из комментария
GUIDE = f"https://github.com/{REPO}/blob/main"


def out(key, val):
    with open(os.environ.get("GITHUB_OUTPUT", "/dev/stdout"), "a") as f:
        f.write(f"{key}={val}\n")


def read(path):
    try:
        with open(path, encoding="utf-8") as f:
            return f.read()
    except OSError:
        return ""


def run(*cmd):
    r = subprocess.run(cmd, capture_output=True, text=True)
    return r.returncode, (r.stdout + r.stderr).strip()


# ---------------------------------------------------------------- карточки ответов

STEPS = ["КМ-1 · схема", "КМ-2 · ПЗ1", "КМ-3 · программы и ПЗ2"]


def progress(cur):
    """Строка «✅ КМ-1 → ▶️ КМ-2 → ⬜ КМ-3»: шаги до cur — сделаны, cur — текущий."""
    return " → ".join(f"{'✅' if i < cur else '▶️' if i == cur else '⬜'} {s}" for i, s in enumerate(STEPS, 1))


def card(step, title, files="", student="", ai="", nxt=""):
    """Ответ бота: заголовок, ход работы, 📎 файлы, 🧑‍🎓 студенту, 🤖 нейронке, ➡️ дальше."""
    parts = [f"## {title}"]
    if step:
        parts.append(f"<sub>{progress(step)}</sub>")
    for head, body in (("📎 Файлы", files), ("🧑‍🎓 Студенту", student), ("🤖 Нейронке", ai), ("➡️ Дальше", nxt)):
        if body:
            parts.append(f"**{head}**\n\n{body}")
    return "\n\n".join(parts)


def raw(nick, rel):
    return f"https://raw.githubusercontent.com/{REPO}/main/students/{nick}/{rel}"


def welcome(nick, fio, group, m):
    return card(1, f"👋 {fio}, {group}, вариант {m} — курсач заведён",
                student="Курсач идёт **тремя шагами**, по одной команде на шаг — пишите её комментарием сюда:\n\n"
                        "| Шаг | Команда | Что получится |\n| --- | --- | --- |\n"
                        "| 1 · КМ-1 | — (уже собирается) | схема Э3 и перечень → отправить руководителю |\n"
                        "| 2 · КМ-2 | `/км2` | ПЗ1 «Аппаратная часть»: дописать 2 абзаца |\n"
                        "| 3 · КМ-3 | `/км3` | три программы + ПЗ2: проверка в эмуляторе, файлы для робота |\n\n"
                        "Каждый ответ бота говорит, что сделать сейчас и какая команда следующая. Результаты — здесь и на Яндекс-диске.\n"
                        "Замечание руководителя — `/замечание КМ-1 <дословно>`. Все команды — `/помощь`.",
                ai=f"Работай по шагам этого Issue. Сначала прочитай [README — промпт]({GUIDE}/README.md) и [AGENTS.md]({GUIDE}/AGENTS.md). "
                   f"Файлы студента — `students/{nick}/` в репо. Присылай файлы **целиком**, блоком ``` с именем файла в первой строке "
                   "(пример — в ответах на `/км2` и `/км3`).",
                nxt="Дождитесь карточки **КМ-1 — схема готова** (пара минут).")


def help_text(nick):
    return card(0, "📖 Как работать в этом Issue",
                student="| Шаг | Команда | Что делает |\n| --- | --- | --- |\n"
                        "| 1 · КМ-1 | `/км1` | пересобрать схему и перечень |\n"
                        "| 2 · КМ-2 | `/км2` | заготовка ПЗ1 и сборка; `/км2` + файл `pz1.md` — ваш текст |\n"
                        "| 3 · КМ-3 | `/км3` | заготовки программ и ПЗ2; `/км3` + файлы `prog1.a51`, `pz2.md` … — проверка и сборка |\n"
                        "| замечания | `/замечание КМ-1 текст` | записать замечание руководителя (дословно) |\n"
                        "| правки схемы | `/правка` + файл `fixes.yaml` | номиналы, перечень, «Примечание», сдвиги блоков |",
                ai="Файл присылается блоком ``` сразу под командой; **в первой строке блока — имя файла**:\n\n"
                   "````\n/км3\n```prog1.a51\n; весь файл программы 1\n```\n```pz2.md\n…весь текст pz2.md…\n```\n````\n\n"
                   f"Файлы студента: `students/{nick}/` (читать: raw-ссылки в ответах на `/км2`, `/км3`). "
                   f"Гайды: [код]({GUIDE}/docs/code-guide.md) · [ПЗ]({GUIDE}/docs/pz-guide.md) · "
                   f"[правки и замечания]({GUIDE}/AGENTS.md#замечания-руководителя-после-сдачи-км).")


# ---------------------------------------------------------------- форма и команды

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


def nick_of(group, fio):
    code, o = run("bin/mpsgen", "-nick", "-group", group, "-name", fio)
    return o if code == 0 else ""


def parse(text):
    """Команды комментария: [(команда, аргументы, [(инфо-строка блока, содержимое), …])] — все блоки до следующей команды."""
    lines = text.splitlines()
    res, i = [], 0
    while i < len(lines):
        m = re.match(r"^\s*/(\S+)\s*(.*)$", lines[i])
        if not m:
            i += 1
            continue
        res.append((m.group(1).lower(), m.group(2).strip(), []))
        i += 1
        while i < len(lines) and not re.match(r"^\s*/\S", lines[i]):
            fm = re.match(r"^\s*(`{3,})(.*)$", lines[i])
            if not fm:
                i += 1
                continue
            fence, info = fm.group(1), fm.group(2).strip()
            body, k = [], i + 1
            while k < len(lines) and not lines[k].strip().startswith(fence):
                body.append(lines[k])
                k += 1
            res[-1][2].append((info, "\n".join(body) + "\n"))
            i = k + 1
    return res


ALLOWED = re.compile(r"^(code/[\w.-]+\.a51|pz/[\w.-]+\.md|pz/flow/[\w.-]+\.flow|schema/[\w.-]+\.yaml|[\w.-]+\.md)$")


def safe_rel(rel):
    """Путь внутри папки студента: только известные места и расширения, без «..»."""
    rel = rel.strip().strip("`").lstrip("./")
    if ".." in rel.split("/") or not ALLOWED.match(rel):
        raise ValueError(f"путь «{rel}» нельзя: можно code/*.a51, pz/*.md, pz/flow/*.flow, schema/*.yaml, *.md")
    return rel


def path_of(info, default=""):
    """Имя файла из первой строки блока (```prog1.a51, ```asm prog1.a51, ```pz2.md) → путь в папке студента."""
    words = info.replace(",", " ").split()
    name = next((w for w in words if "." in w or w in ("prog1", "prog2", "prog3")), "")
    if not name:
        if default:
            return default
        raise ValueError("у блока нет имени файла: в первой строке блока напишите, например, ```prog1.a51 или ```pz2.md")
    base = name.split("/")[-1]
    if re.fullmatch(r"prog[123](\.a51)?", base):
        return f"code/{base.split('.')[0]}.a51"
    if base in ("pz1.md", "pz2.md"):
        return f"pz/{base}"
    if base.endswith(".flow"):
        return f"pz/flow/{base}"
    if base == "fixes.yaml":
        return "schema/fixes.yaml"
    return safe_rel(name)


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
    base = os.path.realpath(os.path.join(ROOT, "students", nick))
    path = os.path.join(ROOT, "students", nick, rel)
    if os.path.commonpath([os.path.realpath(path), base]) != base:
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


def programs(nick):
    """Три программы варианта — из params.md (генератор без схемы, быстро)."""
    if run("bin/mpsgen", "-student", f"students/{nick}/variant.yaml", "-lib", "", "-outroot", "build")[0] != 0:
        return []
    for p in glob.glob("build/*/*/params.md"):
        res = [f"{m.group(1)}. {m.group(2)}" for m in re.finditer(r"^\|\s*([123])\s*\|\s*(.+?)\s*\|\s*$", read(p), re.M)]
        if res:
            return res
    return []


# ---------------------------------------------------------------- шаги

def step_km2(nick, blocks, changed, dispatch, reply, sdir):
    for info, content in blocks:
        rel = path_of(info, default="pz/pz1.md")
        if rel != "pz/pz1.md":
            raise ValueError(f"в КМ-2 присылается только `pz1.md`, а не `{rel}` (программы и ПЗ2 — в `/км3`)")
        write(nick, rel, content, changed)
    if blocks:
        dispatch.append("km2")
        reply.append("📥 Получено: `pz1.md` — собираю ПЗ1.")
        return
    if not os.path.exists(os.path.join(sdir, "pz", "pz1.md")):
        if run("scripts/schema_fetch.sh", nick)[0] != 0 or run("make", "pz1-init", f"S={nick}")[0] != 0:
            raise ValueError("заготовку ПЗ1 делаю по готовой схеме — дождитесь карточки КМ-1 и напишите `/км2` ещё раз")
        changed.append(f"students/{nick}/pz/pz1.md")
    dispatch.append("km2")
    reply.append(card(2, "📝 Шаг 2 · КМ-2 — ПЗ1 «Аппаратная часть»",
                      files=f"Заготовка текста: [`pz/pz1.md`]({raw(nick, 'pz/pz1.md')}) — в ней 2 места с подсказками «✍».",
                      student="Почти вся ПЗ1 собирается сама: схемы, расчёты, таблицы, временные диаграммы, перечень. "
                              "Вам — **два абзаца**:\n"
                              "- `p1.intro` — введение: что делает ваша система и чем вариант отличается (1 абзац);\n"
                              "- `p1.buffer` — как внешнее устройство передаёт X2 через буфер, какое прерывание, кто двигает "
                              "«голову» и «хвост» (3–5 предложений).\n\n"
                              "Сейчас соберётся ПЗ1 как есть — с жёлтыми «ДОПИШИ» на этих местах: посмотрите, как выглядит.",
                      ai=f"Прочитай заготовку [`pz1.md`]({raw(nick, 'pz/pz1.md')}) и [гайд по ПЗ]({GUIDE}/docs/pz-guide.md) "
                         "(ПЗ1 и «Своими словами»). Напиши тексты под подсказками от лица студента, своими словами, и пришли файл **целиком**:\n\n"
                         "````\n/км2\n```pz1.md\n…весь файл pz1.md с заполненными разделами…\n```\n````",
                      nxt="Пришлите заполненный `pz1.md` командой `/км2` — придёт карточка ПЗ1. Потом шаг 3: `/км3`."))


def step_km3(nick, blocks, changed, dispatch, reply, sdir):
    for info, content in blocks:
        rel = path_of(info)
        if not (rel.startswith("code/") or rel == "pz/pz2.md" or rel.startswith("pz/flow/")):
            raise ValueError(f"в КМ-3 присылаются программы, `pz2.md` и схемы алгоритмов `*.flow`, а не `{rel}`")
        write(nick, rel, content, changed)
    if blocks:
        dispatch.append("km3")
        reply.append("📥 Получено: " + ", ".join(f"`{path_of(i).split('/')[-1]}`" for i, _ in blocks) +
                     " — проверяю программы в эмуляторе и собираю ПЗ2.")
        return
    if all(os.path.exists(os.path.join(sdir, "code", f"prog{n}.a51")) for n in (1, 2, 3)):
        dispatch.append("km3")
        reply.append("⏳ Перепроверяю программы и пересобираю ПЗ2 по последним файлам.")
        return
    for target in ("code-init", "pz2-init"):
        code, o = run("make", target, f"S={nick}")
        if code != 0:
            raise ValueError(f"`make {target}`:\n```\n{o[-1500:]}\n```")
    changed.append(f"students/{nick}/")
    progs = programs(nick)
    plist = "\n".join(f"   {p}" for p in progs) if progs else "   (список — в `params.md` схемы)"
    reply.append(card(3, "💻 Шаг 3 · КМ-3 — программы и ПЗ2",
                      files=" · ".join(f"[`prog{n}.a51`]({raw(nick, f'code/prog{n}.a51')})" for n in (1, 2, 3)) +
                            f" · [`pz2.md`]({raw(nick, 'pz/pz2.md')}) · схемы алгоритмов `students/{nick}/pz/flow/` — заготовки под ваш вариант.",
                      student=f"**Ваши программы:**\n{plist}\n\n"
                              "Заготовки уже по шаблону рис. 7 ТЗ (заглушки прерываний, `%proc%`, `%stop%`) — осталось написать сами процедуры. "
                              "Каждую присланную версию бот прогоняет в эмуляторе 8051 на модели вашей схемы и отвечает отчётом ✅/❌; "
                              "по ✅ — готовые файлы для робота и ПЗ2.",
                      ai=f"1. Прочитай [гайд по коду]({GUIDE}/docs/code-guide.md) — **раздел 0 (рецепт)**, правила робота и что видит программа на этой схеме.\n"
                         "2. Возьми заготовки (ссылки выше) и `vars.inc` варианта (ссылка в карточке КМ-1), напиши **свою** реализацию каждой программы "
                         "(не копируй чужие `students/*/code/` и `internal/codecheck/testdata/ref/`).\n"
                         f"3. Схемы алгоритмов в `pz/flow/` подгони под свой код ([формат]({GUIDE}/docs/pz-guide.md)), в `pz2.md` заполни места «✍».\n"
                         "4. Пришли всё одним комментарием — каждый файл блоком, **имя файла в первой строке блока**:\n\n"
                         "````\n/км3\n```prog1.a51\n; весь файл\n```\n```prog2.a51\n…\n```\n```prog3.a51\n…\n```\n```pz2.md\n…\n```\n````\n\n"
                         "5. Если в отчёте ❌ — исправь по отчёту и пришли файл снова (можно только изменённые).",
                      nxt="Пришлите программы командой `/км3` — придёт карточка с отчётом, файлами для робота и ПЗ2."))


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
        return finish("", [], [], reply)
    sdir = os.path.join(ROOT, "students", nick)
    own = read(os.path.join(sdir, "issue")).strip()
    if own and own != issue:
        reply.append(f"❌ Папка `students/{nick}/` уже ведётся в Issue #{own}. Работайте там "
                     "(или закройте старый Issue и напишите руководителю репо).")
        return finish("", [], [], reply)

    if event in ("opened", "edited", "reopened"):
        new = variant_yaml(form)
        old = read(os.path.join(sdir, "variant.yaml"))
        strip = lambda s: "\n".join(l for l in s.splitlines() if not l.startswith("#"))
        if strip(new) != strip(old):
            write(nick, "variant.yaml", new, changed)
            dispatch.append("km1" if not old else "km1-next")
        if own != issue:
            write(nick, "issue", issue + "\n", changed)
        if event == "opened" or not old:
            reply.append(welcome(nick, fio, group, m))
        elif changed:
            reply.append("Данные варианта обновлены — пересобираю схему, затем ПЗ1 и ПЗ2 по ней.")
        return finish(nick, changed, dispatch, reply)

    cmds = parse(read(os.environ.get("COMMENT_FILE", "")))
    if not cmds:
        return finish(nick, [], [], [])  # обычное обсуждение — молчим
    if not os.path.exists(os.path.join(sdir, "variant.yaml")):
        write(nick, "variant.yaml", variant_yaml(form), changed)
        write(nick, "issue", issue + "\n", changed)
        dispatch.append("km1")
    for cmd, args, blocks in cmds:
        block = blocks[0][1] if blocks else None
        try:
            if cmd in ("помощь", "help"):
                reply.append(help_text(nick))
            elif cmd in ("км1", "km1", "схема"):
                dispatch.append("km1")
            elif cmd in ("км2", "km2", "пз1", "pz1"):
                step_km2(nick, blocks, changed, dispatch, reply, sdir)
            elif cmd in ("км3", "km3"):
                step_km3(nick, blocks, changed, dispatch, reply, sdir)
            elif cmd in ("всё", "все", "all"):
                dispatch.append("km1-next")
            elif cmd in ("заготовки", "init"):
                step_km3(nick, [], changed, dispatch, reply, sdir)
                if not os.path.exists(os.path.join(sdir, "pz", "pz1.md")):
                    step_km2(nick, [], changed, dispatch, reply, sdir)
            elif cmd in ("пз2", "pz2"):
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
                    raise ValueError("/правка: нужен блок ```fixes.yaml с правками (формат — AGENTS.md, «Замечания руководителя»)")
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
                who, owner = os.environ.get("COMMENT_AUTHOR", ""), os.environ.get("REPO_OWNER", "")
                reply.append(f"@{owner}: **@{who}** просит права на запись в репо (чтобы работать пушами и Run workflow).\n\n"
                             f"Добавить: Settings → Collaborators → Add people → `{who}` (роль Write), или одной командой:\n"
                             f"```sh\ngh api -X PUT repos/{REPO}/collaborators/{who} -f permission=push\n```\n"
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
                reply.append("📌 Замечание записано в историю (`remarks.md`), следующая версия файла для отправки будет на номер больше.\n\n"
                             "Если правка касается только вашей схемы (номинал, перечень, «Примечание») — `/правка` + блок ```fixes.yaml "
                             f"([формат]({GUIDE}/AGENTS.md#замечания-руководителя-после-сдачи-км)); текст ПЗ — `/км2` / `/км3` с файлом; "
                             "ошибка самой схемы (связи) — напишите об этом здесь.")
            else:
                reply.append(f"Не знаю команду `/{cmd}` — `/помощь`.")
        except ValueError as e:
            reply.append(f"❌ {e}")
    finish(nick, changed, dispatch, reply)


def finish(nick, changed, dispatch, reply):
    out("nick", nick)
    out("commit", "1" if changed else "")
    dispatch = [k for k in dispatch if k]
    d = []
    for k in ("km1-next", "km1", "km2", "km3"):
        if k in dispatch and k not in d:
            d.append(k)
    if "km1-next" in d:
        d = ["km1-next"]
    elif "km1" in d:
        d = [k for k in d if k != "km2"]  # ПЗ1 по старой схеме не нужна
    if d:
        names = {"km1-next": "схема → ПЗ1 и ПЗ2", "km1": "схема (КМ-1)", "km2": "ПЗ1 (КМ-2)", "km3": "программы и ПЗ2 (КМ-3)"}
        reply.append("⏳ Запущено: " + ", ".join(names[k] for k in d) + ". Результат придёт сюда карточкой.")
    out("dispatch", " ".join(d))
    path = os.path.join(os.environ.get("RUNNER_TEMP", "/tmp"), f"reply-{int(time.time() * 1000)}.md")
    with open(path, "w", encoding="utf-8") as f:
        f.write("\n\n".join(reply))
    out("reply", path if reply else "")


if __name__ == "__main__":
    sys.exit(main())
