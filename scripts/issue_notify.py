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
        rep = read(os.path.join(d, "pz1" if run.startswith("ПЗ1") else "pz2", "report.md"))
        m = re.search(r"Мест «ДОПИШИ»: \d+, осталось заполнить: (\d+)", rep)
        if m:
            left = int(m.group(1))
            lines.append("Все места «ДОПИШИ» заполнены." if left == 0 else
                         f"Осталось мест «ДОПИШИ»: **{left}** — дописать в `students/{nick}/pz/` и прислать командой `/пз1` / `/пз2`.")
        code = read(os.path.join(d, ".pub", "программы", "report.md"))
        if code:
            bad = len(re.findall(r"^## ❌", code, re.M))
            lines.append(f"Программы: {'❌ есть ошибки — ' if bad else '✅ '}[отчёт проверки]({blob}/{q('программы/report.md')})"
                         + ("" if bad else "; файлы для робота — `программы/Фамилия ИО-код-n.txt`"))
    disk = read(os.path.join(d, ".disk")) or path
    if PUBLIC:
        lines.append(f"\n📁 Яндекс-диск: [{disk}]({PUBLIC}/{q(disk)}) · все файлы прогона: [results]({tree})")
    else:
        lines.append(f"\n📁 Все файлы прогона: [results]({tree})")
    lines.append(f"<sub>запуск: {RUN}</sub>")
    comment(issue, "\n".join(lines))


def fail(nick):
    issue = issue_of(nick)
    if issue:
        comment(issue, f"### ❌ {WF}: не получилось\nЧто именно — в логе: {RUN} (раскройте шаг с красным крестиком).\n"
                       "Частое: «схемы ещё нет» → `/схема`; «схема устарела» → `/всё`.")


if __name__ == "__main__":
    mode, args = sys.argv[1], sys.argv[2:]
    for a in args:
        (ok if mode == "ok" else fail)(a.rstrip("/"))
