#!/usr/bin/env python3
"""Комментарий в Issue студента о результате джобы КМ-1/2/3 (если папка студента ведётся через Issue: students/<ник>/issue).

  scripts/issue_notify.py ok   build/<группа>/<M> [...]   — после публикации: что готово, ссылки на файлы и на диск
  scripts/issue_notify.py fail <ник> [...]                — джоба упала: ссылка на лог

Нужны GH_TOKEN (GITHUB_TOKEN с issues: write), GITHUB_REPOSITORY; ссылка на диск — YADISK_PUBLIC.
Папка прогона — из build/…/.pubdest и .run (publish.sh), на диске — .disk (yadisk_upload.py, если номер сдвинулся).
"""
import json
import os
import re
import subprocess
import sys
import urllib.parse

REPO = os.environ.get("GITHUB_REPOSITORY", "")
RUN = f"{os.environ.get('GITHUB_SERVER_URL', 'https://github.com')}/{REPO}/actions/runs/{os.environ.get('GITHUB_RUN_ID', '')}"
PUBLIC = os.environ.get("YADISK_PUBLIC", "").rstrip("/")
WF = os.environ.get("GITHUB_WORKFLOW", "")


def read(p):
    try:
        with open(p, encoding="utf-8") as f:
            return f.read().strip()
    except OSError:
        return ""


def issue_of(nick):
    n = read(os.path.join("students", nick, "issue"))
    return n if n.isdigit() else ""


def comment(issue, body):
    subprocess.run(["gh", "issue", "comment", issue, "--repo", REPO, "--body-file", "-"], input=body, text=True, check=False)


def q(path):
    return urllib.parse.quote(path)


def ok(d):
    nick = read(os.path.join(d, ".nick"))
    if not nick:
        try:
            nick = json.load(open(os.path.join(d, "meta.json"), encoding="utf-8")).get("nick", "")
        except (OSError, ValueError):
            nick = ""
    issue = issue_of(nick) if nick else ""
    dest, run = read(os.path.join(d, ".pubdest")), read(os.path.join(d, ".run"))
    if not issue or not dest or not run:
        return
    path = f"{dest}/{run}"
    blob = f"https://github.com/{REPO}/blob/results/{q(path)}"
    tree = f"https://github.com/{REPO}/tree/results/{q(path)}"
    raw = f"https://raw.githubusercontent.com/{REPO}/results/{q(path)}"
    files = sorted(os.listdir(os.path.join(d, ".pub"))) if os.path.isdir(os.path.join(d, ".pub")) else []
    lines = []
    if run.startswith("СХЕМА"):
        lines.append(f"### ✅ {run} — схема и перечень (КМ-1)")
        erc = read(os.path.join(d, ".pub", "erc-summary.txt"))
        lines.append("ERC: чисто" if not erc else "⚠ ERC — есть замечания, см. `erc-summary.txt`")
        lines.append(f"\n[![схема]({raw}/schematic.png)]({raw}/schematic.png)\n")
        lines.append(" · ".join(f"[{t}]({blob}/{q(f)})" for t, f in [("схема PDF", "schematic.pdf"), ("KiCad", "schematic.kicad_sch"),
                                                                     ("перечень ПЭ3", "perechen.pdf"), ("перечень для КМ-1", "perechen-km1.pdf"),
                                                                     ("параметры варианта", "params.md"), ("vars.inc", "vars.inc")] if f in files))
    else:
        km = "КМ-2" if run.startswith("ПЗ1") else "КМ-3"
        lines.append(f"### ✅ {run} ({km})")
        docs = [f for f in files if f.endswith(".docx") or f.endswith(".pdf")]
        if docs:
            lines.append(" · ".join(f"[{f}]({blob}/{q(f)})" for f in docs))
        doc, cmd = ("pz1", "пз1") if run.startswith("ПЗ1") else ("pz2", "пз2")
        rep = read(os.path.join(d, doc, "report.md"))
        m = re.search(r"Мест «ДОПИШИ»: \d+, осталось заполнить: (\d+)", rep)
        if m:
            left = int(m.group(1))
            lines.append("Все места «ДОПИШИ» заполнены." if left == 0 else
                         f"Осталось мест «ДОПИШИ»: **{left}** — дописать в `students/{nick}/pz/{doc}.md` и прислать командой `/{cmd}` + текст файла.")
        code = read(os.path.join(d, ".pub", "программы", "report.md"))
        if code:
            bad = len(re.findall(r"^## ❌", code, re.M))
            lines.append(f"Программы: {'❌ есть ошибки — ' if bad else '✅ '}[отчёт проверки]({blob}/{q('программы/report.md')})")
            pd = os.path.join(d, ".pub", "программы")
            robot = sorted(f for f in os.listdir(pd) if f.endswith(".txt"))
            src = sorted(f for f in os.listdir(pd) if f.endswith(".a51"))
            if robot:
                lines.append("- файлы для робота (`vars.inc` уже вклеен): " + " · ".join(f"[{f}]({blob}/{q('программы/' + f)})" for f in robot))
            if src:
                lines.append("- исходники: " + " · ".join(f"[{f}]({blob}/{q('программы/' + f)})" for f in src) +
                             " (листинги `.lst`, `.hex` — в папке прогона)")
    lines += send_hint(d, run, dest, files, blob)
    disk = read(os.path.join(d, ".disk")) or path
    if PUBLIC:
        lines.append(f"\n📁 Яндекс-диск: [{disk}]({PUBLIC}/{q(disk)}) · все файлы прогона: [results]({tree})")
    else:
        lines.append(f"\n📁 Все файлы прогона: [results]({tree})")
    lines.append(f"<sub>запуск: {RUN}</sub>")
    comment(issue, "\n".join(lines))


def send_hint(d, run, dest, files, blob):
    """Что и как отправлять — по ТЗ-2026 (разд. 2.1, 2.2): тема «МПС-…», файл «Фамилия ИО Смысл-vN», в теле — группа и вариант."""
    sv = read(os.path.join(d, ".send")).splitlines()
    if len(sv) < 2:
        return []
    short, ver = sv[0], sv[1]
    grp, rest = (dest.split("/", 1) + [""])[:2]
    var = rest.split(" ", 1)[0]
    body = f"в теле письма — «группа {grp}, вариант {var}»"
    if run.startswith("СХЕМА"):
        f = f"{short} Схема-v{ver}.png"
        link = f"[{f}]({blob}/{q(f)})" if f in files else f"`{f}`"
        return [f"\n✉️ **Сдать КМ-1:** на почту руководителя, тема «МПС-Схема», вложение {link}; {body}."]
    own = ("\n## ⚠️ ОБЯЗАТЕЛЬНО ПЕРЕЧИТАЙТЕ ВЕСЬ ОТЧЁТ ЦЕЛИКОМ ПЕРЕД СДАЧЕЙ\n"
           "**В нём есть жёлтые пометки «ДОПИШИ» — места, которые нужно написать самому. Отчёт с такими пометками сдавать нельзя.**\n"
           "\n✍ **Перед сдачей:** допишите все «ДОПИШИ», затем по последней сборке откройте docx и **перескажите своими словами** общие абзацы "
           "(введение, описания, выводы) — тексты у всех из одного шаблона. Числа, таблицы, рисунки и обозначения не трогать; "
           "следующая сборка перезапишет docx — сохраните копию. Подробно — `docs/pz-guide.md`, «Своими словами».")
    if run.startswith("ПЗ1"):
        return [own, f"\n✉️ **Сдать КМ-2:** допишите «ДОПИШИ», откройте docx → «Сохранить как PDF» → `{short} ПЗ1-v{ver}.pdf`; "
                f"на почту руководителя, тема «МПС-ПЗ1»; {body}."]
    out = [own, f"\n✉️ **Сдать ПЗ2:** docx → «Сохранить как PDF» → `{short} ПЗ2-v{ver}.pdf`; на почту ОСЭП руководителя, тема «МПС-ПЗ2»; {body}."]
    codes = sorted(f for f in os.listdir(os.path.join(d, ".pub", "программы")) if f.endswith(".txt")) if os.path.isdir(os.path.join(d, ".pub", "программы")) else []
    if codes:
        out.append("✉️ **Программы** — после того как ПЗ2 рассмотрят: тема «МПС-код», вложения " +
                   ", ".join(f"`{c}`" for c in codes) + f" (имена не менять — по ним проверяет робот); {body}.")
    out.append(km3_guide(d, var))
    return out


def km3_guide(d, var):
    """Памятка КМ-3: что сдаётся, какие программы у варианта (params.md), как проверяем мы и как сдавать (ТЗ-2026, разд. 2.2)."""
    progs = []
    for line in read(os.path.join(d, "params.md")).splitlines():
        m = re.match(r"^\|\s*([123])\s*\|\s*(.+?)\s*\|\s*$", line)
        if m:
            progs.append(f"   {m.group(1)}. {m.group(2)}")
    lines = ["\n<details><summary><b>📘 КМ-3 — что это и как сдавать (раскрыть)</b></summary>\n",
             "**Что сдаётся:** три программы на ассемблере 8051 по табл. 1 ТЗ и ПЗ2 — описание программной части (по этим программам)."]
    if progs:
        lines.append(f"\n**Твои программы (вариант {var}):**\n" + "\n".join(progs))
    lines += [
        "\n**Что даётся:** заготовки по рис. 7 ТЗ под вариант (`/заготовки` → `code/prog1-3.a51`), `vars.inc` с адресами, битами и "
        "константами варианта (в файлы для робота вклеивается сам), гайд — `docs/code-guide.md` (правила робота, устройства схемы глазами программы).",
        "\n**Требования ТЗ к каждой программе:** заглушки `nop` + `reti` на всех неиспользуемых прерываниях; инициализация сразу после Reset; "
        "процедура задачи с комментарием `%proc%`, вызванная `call` (без бесконечных циклов внутри); последняя команда `jmp $ ; %stop%`; "
        "комментарии — по смыслу, а не перевод мнемоник; у каждой процедуры — комментарий о назначении, входах и выходах.",
        "\n**Как проверяем мы** (каждый прогон): свой ассемблер (байты = MCU 8051 IDE) и эмулятор 8051 с моделью **твоей** схемы "
        "(клавиатура, буфер IDT7005, индикатор, регистр Y2, стробы, дешифратор): сценарии из ТЗ, такты таймеров, правила рис. 7 → отчёт ✅/❌ выше. "
        "Это наша проверка, не робот кафедры: как именно проверяет робот, неизвестно — его ответ главный.",
        "\n**Как прислать код сюда:** комментарий `/prog1` и сразу под ним весь файл блоком ```` ```asm … ``` ```` (так же `/prog2`, `/prog3`) — "
        "проверка и ПЗ2 пересоберутся сами.",
        "\n**Как сдавать (ТЗ, разд. 2.2):**\n"
        "   1. Сначала **ПЗ2** — PDF на почту ОСЭП руководителя, тема «МПС-ПЗ2».\n"
        "   2. **После рассмотрения ПЗ2** (с учётом замечаний) — **программы**: файлы для робота (ссылки выше, `.txt`, UTF-8) на почту ОСЭП "
        "руководителя, тема «МПС-код», имена «Фамилия ИО-код-n» не менять, в теле — группа и вариант.\n"
        "   3. Проверка автоматическая — робот кафедры в сети МЭИ `10.3.170.2:8051` (работает после 7-й недели); ответ «принято» или ошибка. "
        "Ответ робота запишите сюда: `/замечание КМ-3 <что ответил робот>` — сверим с нашей проверкой.",
        "\n</details>"]
    return "\n".join(lines)


def fail(nick):
    issue = issue_of(nick)
    if issue:
        comment(issue, f"### ❌ {WF}: не получилось\nЧто именно — в логе: {RUN} (раскройте шаг с красным крестиком).\n"
                       "Частое: «схемы ещё нет» → `/схема`; «схема устарела» → `/всё`.")


if __name__ == "__main__":
    mode, args = sys.argv[1], sys.argv[2:]
    for a in args:
        (ok if mode == "ok" else fail)(a.rstrip("/"))
