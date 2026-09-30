#!/usr/bin/env bash
# Выкладывает результаты сборки в ветку results и пишет сводку запуска (GitHub Actions).
# Использование: scripts/publish.sh build/<группа>/<вариант> [...]
#
# Ветка results — только сгенерированные файлы: <группа>/<вариант>/прогон-N/{schematic.png,pdf,…, params.md, vars.inc,
# code/ — проверка программ, run.txt — откуда прогон} + README.md с оглавлением. Коммит один (история ветки не нужна),
# но прошлые прогоны не перетираются: каждый запуск кладёт вариант в новую папку прогон-N (N = последний + 1).
# Номер пишется в build/<группа>/<вариант>/.run — по нему yadisk_upload.py кладёт на диск в папку с тем же номером.
# Файлы в корне варианта (раскладка до прогонов) при первой публикации переезжают в прогон-1.
# SUMMARY=0 — не писать сводку запуска (её пишет mpscode).
set -euo pipefail

# только папки, где генератор отработал
dirs=()
for d in "$@"; do { [ -f "$d/params.md" ] || [ -d "$d/code" ]; } && dirs+=("${d%/}") || echo "пропуск $d: нет params.md и code/" >&2; done
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
    # раскладка до прогонов: файлы прямо в папке варианта → прогон-1
    if ! ls -d "$v"/прогон-* >/dev/null 2>&1 && [ -n "$(ls -A "$v")" ]; then
      mkdir "$v/прогон-1" && find "$v" -mindepth 1 -maxdepth 1 ! -name прогон-1 -exec mv {} "$v/прогон-1/" \;
    fi
    last="$(ls -d "$v"/прогон-* 2>/dev/null | sed 's|.*/прогон-||' | sort -n | tail -1)"
    n=$(( ${last:-0} + 1 ))
    out="$v/прогон-$n"
    mkdir -p "$out"
    for f in "${keep[@]}"; do [ -f "$d/$f" ] && cp "$d/$f" "$out/"; done
    [ -d "$d/code" ] && cp -r "$d/code" "$out/code"
    printf 'прогон %s\nworkflow: %s, запуск %s\nкоммит: %s\nдата: %s\n' "$n" "${GITHUB_WORKFLOW:-local}" \
      "${GITHUB_SERVER_URL:-}/${repo}/actions/runs/${GITHUB_RUN_ID:-}" "${GITHUB_SHA:-}" "$(date -u '+%Y-%m-%d %H:%M UTC')" > "$d/run.txt"
    cp "$d/run.txt" "$out/"
    echo "$n" > "$d/.run"
  done
  # оглавление
  {
    echo "# Результаты генератора"
    echo
    echo "Ветка обновляется автоматически (Actions → build). Руками не править. Исходники — ветка \`main\`."
    echo
    echo "Каждый запуск кладёт вариант в новую папку \`прогон-N\` — прошлые не перетираются. В таблице — последние."
    echo
    echo "| Группа | Вариант | Прогонов | Схема (последняя) | Перечень | Параметры | ERC | Код КМ-3 (последний) |"
    echo "| --- | --- | --- | --- | --- | --- | --- | --- |"
    (cd "$work/r" && find . -mindepth 3 -maxdepth 3 -type d -name 'прогон-*' | sed 's|^\./||; s|/прогон-[0-9]*$||' |
      sort -u | sort -t/ -k1,1 -k2,2n) | while read -r n; do
      runs="$(ls -d "$work/r/$n"/прогон-* | sed 's|.*/прогон-||' | sort -n)"
      sch="—" pe="—" par="—" e="—" code="—" s="" c=""
      for k in $runs; do
        [ -f "$work/r/$n/прогон-$k/params.md" ] && s="$k"
        [ -f "$work/r/$n/прогон-$k/code/report.md" ] && c="$k"
      done
      if [ -n "$s" ]; then
        p="$n/прогон-$s"
        sch="[прогон $s]($p): [PNG]($p/schematic.png) · [PDF]($p/schematic.pdf) · [KiCad]($p/schematic.kicad_sch)"
        pe="[ПЭ3]($p/perechen.pdf) · [КМ-1]($p/perechen-km1.pdf)"
        par="[params.md]($p/params.md) · [vars.inc]($p/vars.inc)"
        e="чисто"; [ -s "$work/r/$p/erc-summary.txt" ] && e="⚠ есть замечания"
      fi
      if [ -n "$c" ]; then
        code="[прогон $c: отчёт]($n/прогон-$c/code/report.md)"; grep -q "^## ❌" "$work/r/$n/прогон-$c/code/report.md" && code="❌ $code"
      fi
      echo "| ${n%%/*} | ${n#*/} | [$(echo "$runs" | wc -l | tr -d ' ')]($n) | $sch | $pe | $par | $e | $code |"
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
  [ -f "$d/params.md" ] || continue
  n="${d#build/}/прогон-$(cat "$d/.run")"
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
