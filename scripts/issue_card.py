"""Карточка-комментарий в Issue курсача — общая для бота (issue_bot.py) и уведомлений джоб (issue_notify.py).

Вид (GitHub markdown, шрифт менять нельзя — разделение цветными плашками и сворачиванием):
  ## заголовок + строка прогресса шагов
  ### 📎 Файлы — список ссылок
  [!TIP]       🧑‍🎓 Вам — что сделать сейчас      (зелёная)
  [!CAUTION]   ⚠️ Перед сдачей — обязательно       (красная, только где нужно)
  [!IMPORTANT] ➡️ Дальше                          (фиолетовая)
  доп. блок (например, памятка КМ-3)
  <details> 🤖 Для нейронки — инструкция           (свёрнуто: студенту не мешает, нейронка читает целиком)
"""

STEPS = ["КМ-1 · схема", "КМ-2 · ПЗ1", "КМ-3 · программы и ПЗ2"]


def progress(cur):
    """«✅ КМ-1 → ▶️ КМ-2 → ⬜ КМ-3»: шаги до cur — сделаны, cur — текущий."""
    return " → ".join(f"{'✅' if i < cur else '▶️' if i == cur else '⬜'} {s}" for i, s in enumerate(STEPS, 1))


def alert(kind, title, body):
    """Цветная плашка GitHub (> [!TIP] …): каждая строка — в цитате, заголовок жирным."""
    lines = [f"> [!{kind}]", f"> **{title}**", ">"]
    lines += [f"> {l}" if l.strip() else ">" for l in body.strip("\n").split("\n")]
    return "\n".join(lines)


def card(step, title, files="", student="", ai="", nxt="", warn="", extra="", foot=""):
    parts = [f"## {title}"]
    if step:
        parts.append(f"<sub>{progress(step)}</sub>")
    if files:
        parts.append("### 📎 Файлы\n\n" + files)
    if student:
        parts.append(alert("TIP", "🧑‍🎓 Вам — что сделать сейчас", student))
    if warn:
        parts.append(alert("CAUTION", "⚠️ Перед сдачей — обязательно", warn))
    if nxt:
        parts.append(alert("IMPORTANT", "➡️ Дальше", nxt))
    if extra:
        parts.append(extra)
    if ai:
        parts.append("<details>\n<summary>🤖 <b>Для нейронки</b> — инструкция (раскрыть)</summary>\n\n" + ai.strip() + "\n\n</details>")
    if foot:
        parts.append(foot)
    return "\n\n".join(parts)
