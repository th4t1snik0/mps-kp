#!/usr/bin/env bash
# Выкладывает результаты сборки в ветку results и пишет сводку запуска (GitHub Actions).
# Использование: RUN_KIND=СХЕМА|ПЗ1|ПЗ2 scripts/publish.sh build/<группа>/<вариант> [...]
#
# Ветка results — только сгенерированные файлы: <группа>/<вариант>/<ВИД>-N/ + README.md с оглавлением.
# У каждого вида свой счётчик, прошлые прогоны не перетираются:
#   СХЕМА-N — КМ-1: схема PNG/PDF/KiCad, перечни, params.md, vars.inc, ERC;
#   ПЗ1-N   — КМ-2: «Фамилия ИО ПЗ1.docx» и рисунки;
#   ПЗ2-N   — КМ-3: «Фамилия ИО ПЗ2.docx», рисунки схем алгоритмов, программы/ (файлы для робота, отчёт проверки).
# Готовая папка копируется в build/<группа>/<вариант>/.pub, её имя — в .run: yadisk_upload.py заливает её на диск как есть.
# SUMMARY=0 — не писать сводку запуска (её пишет mpscode).
set -euo pipefail
kind="${RUN_KIND:?нужен RUN_KIND: СХЕМА, ПЗ1 или ПЗ2}"
case "$kind" in СХЕМА|ПЗ1|ПЗ2) ;; *) echo "RUN_KIND: $kind — ждём СХЕМА, ПЗ1 или ПЗ2" >&2; exit 1 ;; esac

# только папки, где генератор отработал
dirs=()
for d in "$@"; do { [ -f "$d/params.md" ] || [ -d "$d/code" ] || [ -d "$d/pz1" ] || [ -d "$d/pz2" ]; } && dirs+=("${d%/}") || echo "пропуск $d: нет результатов" >&2; done
[ ${#dirs[@]} -gt 0 ] || { echo "нечего публиковать" >&2; exit 0; }
set -- "${dirs[@]}"

repo="${GITHUB_REPOSITORY:?нужен GITHUB_REPOSITORY}"
raw="https://raw.githubusercontent.com/$repo/results"
tree="https://github.com/$repo/blob/results"
summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
keep=(params.md params.json vars.inc schematic.png schematic.pdf schematic.kicad_sch schematic.kicad_pro ramka.kicad_wks erc-summary.txt erc.rpt
      perechen.pdf perechen-km1.pdf perechen.md perechen-1.png perechen-2.png perechen-3.png perechen-km1-1.png)

work="$(mktemp -d)"
git config --global user.name "github-actions[bot]"
git config --global user.email "41898282+github-actions[bot]@users.noreply.github.com"

for attempt in 1 2 3; do
  rm -rf "$work/r" "$work/idx0" && mkdir -p "$work/r"
  lease=""
  if git fetch -q origin results 2>/dev/null; then
    lease="$(git rev-parse FETCH_HEAD)"
    GIT_INDEX_FILE="$work/idx0" git --work-tree="$work/r" checkout FETCH_HEAD -- . 2>/dev/null || true
    # старая плоская раскладка (<папка>/params.md) — убрать, теперь только <группа>/<вариант>/
    find "$work/r" -mindepth 2 -maxdepth 2 -name params.md -exec dirname {} \; | xargs -r rm -rf
  fi
  for d in "$@"; do
    name="${d#build/}"
    v="$work/r/$name"
    mkdir -p "$v"
    last="$(ls -d "$v/$kind"-* 2>/dev/null | sed "s|.*/$kind-||" | grep -E '^[0-9]+$' | sort -n | tail -1 || true)"
    n=$(( ${last:-0} + 1 ))
    out="$v/$kind-$n"
    mkdir -p "$out"
    # всё, кроме исходника pandoc (*.md), но с отчётом
    flat() { find "$1" -maxdepth 1 -type f \( ! -name '*.md' -o -name report.md \) -exec cp {} "$2/" \; ; }
    case "$kind" in
      СХЕМА) for f in "${keep[@]}"; do [ -f "$d/$f" ] && cp "$d/$f" "$out/"; done ;;
      ПЗ1)   [ -d "$d/pz1" ] && flat "$d/pz1" "$out" ;;
      ПЗ2)   [ -d "$d/pz2" ] && flat "$d/pz2" "$out"
             if [ -d "$d/code" ]; then mkdir -p "$out/программы" && flat "$d/code" "$out/программы"; fi ;;
    esac
    printf '%s-%s\nworkflow: %s, запуск %s\nкоммит: %s\nдата: %s\n' "$kind" "$n" "${GITHUB_WORKFLOW:-local}" \
      "${GITHUB_SERVER_URL:-}/${repo}/actions/runs/${GITHUB_RUN_ID:-}" "${GITHUB_SHA:-}" "$(date -u '+%Y-%m-%d %H:%M UTC')" > "$d/run.txt"
    cp "$d/run.txt" "$out/"
    echo "$kind-$n" > "$d/.run"
    rm -rf "$d/.pub" && cp -R "$out" "$d/.pub"
  done
  # оглавление
  {
    echo "# Результаты генератора"
    echo
    echo "Ветка обновляется автоматически (Actions → build). Руками не править. Исходники — ветка \`main\`."
    echo
    echo "Каждый запуск джобы — новая папка своего вида: \`СХЕМА-N\` (КМ-1), \`ПЗ1-N\` (КМ-2), \`ПЗ2-N\` (КМ-3: ПЗ2 и программы)."
    echo "Прошлые не перетираются. В таблице — последние."
    echo
    echo "| Группа | Вариант | КМ-1: схема | Перечень | Параметры | ERC | КМ-2: ПЗ1 | КМ-3: ПЗ2 | КМ-3: программы |"
    echo "| --- | --- | --- | --- | --- | --- | --- | --- | --- |"
    (cd "$work/r" && find . -mindepth 2 -maxdepth 2 -type d ! -name '.*' | sed 's|^\./||' | sort -t/ -k1,1 -k2,2n) | while read -r n; do
      lastof() { ls -d "$work/r/$n/$1"-* 2>/dev/null | sed "s|.*/$1-||" | grep -E '^[0-9]+$' | sort -n | tail -1 || true; }
      url() { printf '%s' "$1" | sed 's/ /%20/g'; }
      sch="—" pe="—" par="—" e="—" pz1="—" pz2="—" code="—"
      k="$(lastof СХЕМА)"
      if [ -n "$k" ]; then
        p="$n/СХЕМА-$k"
        sch="[СХЕМА-$k]($p): [PNG]($p/schematic.png) · [PDF]($p/schematic.pdf) · [KiCad]($p/schematic.kicad_sch)"
        pe="[ПЭ3]($p/perechen.pdf) · [КМ-1]($p/perechen-km1.pdf)"
        par="[params.md]($p/params.md) · [vars.inc]($p/vars.inc)"
        e="чисто"; [ -s "$work/r/$p/erc-summary.txt" ] && e="⚠ есть замечания"
      fi
      k="$(lastof ПЗ1)"
      if [ -n "$k" ]; then
        f="$(ls "$work/r/$n/ПЗ1-$k/"*.docx 2>/dev/null | head -1 || true)"
        pz1="[ПЗ1-$k]($(url "$n/ПЗ1-$k/$(basename "${f:-.}")"))"; [ -n "$f" ] || pz1="[ПЗ1-$k]($n/ПЗ1-$k)"
      fi
      k="$(lastof ПЗ2)"
      if [ -n "$k" ]; then
        f="$(ls "$work/r/$n/ПЗ2-$k/"*.docx 2>/dev/null | head -1 || true)"
        pz2="[ПЗ2-$k]($(url "$n/ПЗ2-$k/$(basename "${f:-.}")"))"; [ -n "$f" ] || pz2="[ПЗ2-$k]($n/ПЗ2-$k)"
        r="$work/r/$n/ПЗ2-$k/программы/report.md"
        if [ -f "$r" ]; then
          code="[отчёт]($n/ПЗ2-$k/программы/report.md)"; grep -q "^## ❌" "$r" && code="❌ $code"
        fi
      fi
      echo "| ${n%%/*} | ${n#*/} | $sch | $pe | $par | $e | $pz1 | $pz2 | $code |"
    done
  } > "$work/r/README.md"

  idx="$work/index"
  rm -f "$idx"
  GIT_INDEX_FILE="$idx" git --work-tree="$work/r" add -A .
  t="$(GIT_INDEX_FILE="$idx" git write-tree)"
  c="$(git commit-tree "$t" -m "results: ${GITHUB_RUN_ID:-local} (${GITHUB_SHA:0:7})")"
  args=(--force)
  [ -n "$lease" ] && args=(--force-with-lease="refs/heads/results:$lease")
  if git push -q "${args[@]}" origin "$c:refs/heads/results"; then break; fi
  echo "results: гонка с другим запуском, повтор $attempt" >&2
  sleep 5
done

[ "${SUMMARY:-1}" = "0" ] && exit 0
# сводка на странице запуска; картинка — по коммиту, а не по ветке (raw-CDN кэширует ветку минутами)
raw="https://raw.githubusercontent.com/$repo/$c"
for d in "$@"; do
  [ "$kind" = СХЕМА ] && [ -f "$d/params.md" ] || continue
  n="${d#build/}/$(cat "$d/.run")"
  {
    echo "## $n"
    grep -m1 "^Студент:" "$d/params.md" || true
    if [ -s "$d/erc-summary.txt" ]; then echo "⚠ ERC:"; echo '```'; cat "$d/erc-summary.txt"; echo '```'; else echo "ERC: чисто"; fi
    echo
    echo "[Схема PDF]($tree/$n/schematic.pdf) · [KiCad]($tree/$n/schematic.kicad_sch) · [Перечень ПЭ3]($tree/$n/perechen.pdf) · [Перечень для КМ-1]($tree/$n/perechen-km1.pdf) · [vars.inc]($tree/$n/vars.inc) · [все файлы](https://github.com/$repo/tree/results/$n)"
    echo
    echo "[![схема]($raw/$n/schematic.png)]($raw/$n/schematic.png)"
    echo
    echo "<details><summary>Параметры варианта</summary>"
    echo
    cat "$d/params.md"
    echo
    echo "</details>"
    echo
  } >> "$summary"
done
