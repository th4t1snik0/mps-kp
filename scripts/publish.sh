#!/usr/bin/env bash
# Выкладывает результаты сборки в ветку results и пишет сводку запуска (GitHub Actions).
# Использование: scripts/publish.sh build/<группа>/<вариант> [...]
#
# Ветка results — только сгенерированные файлы: <группа>/<вариант>/{schematic.png,pdf,kicad_sch,…, params.md, vars.inc,
# code/ — проверка программ} + README.md с оглавлением. Каждый раз перезаписывается одним коммитом (история не нужна).
# Папки и файлы, которых нет в этом запуске, сохраняются из прошлого содержимого ветки: workflow build обновляет
# схему и не трогает code/, workflow code — наоборот. SUMMARY=0 — не писать сводку запуска (её пишет mpscode).
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
    mkdir -p "$work/r/$name"
    if [ -f "$d/params.md" ]; then
      for f in "${keep[@]}"; do rm -f "$work/r/$name/$f"; [ -f "$d/$f" ] && cp "$d/$f" "$work/r/$name/"; done
    fi
    if [ -d "$d/code" ]; then rm -rf "$work/r/$name/code" && cp -r "$d/code" "$work/r/$name/code"; fi
  done
  # оглавление
  {
    echo "# Результаты генератора"
    echo
    echo "Ветка обновляется автоматически (Actions → build). Руками не править. Исходники — ветка \`main\`."
    echo
    echo "| Группа | Вариант | Схема | Перечень | Параметры | Асм | ERC | Код (КМ-3) |"
    echo "| --- | --- | --- | --- | --- | --- | --- | --- |"
    (cd "$work/r" && { find . -mindepth 3 -maxdepth 3 -name params.md; find . -mindepth 4 -maxdepth 4 -path '*/code/report.md'; } |
      sed 's|^\./||; s|/params.md$||; s|/code/report.md$||' | sort -u | sort -t/ -k1,1 -k2,2n) | while read -r n; do
      sch="—" pe="—" par="—" asm="—" e="—" code="—"
      if [ -f "$work/r/$n/params.md" ]; then
        sch="[PNG]($n/schematic.png) · [PDF]($n/schematic.pdf) · [KiCad]($n/schematic.kicad_sch)"
        pe="[ПЭ3]($n/perechen.pdf) · [КМ-1]($n/perechen-km1.pdf)"
        par="[params.md]($n/params.md)" asm="[vars.inc]($n/vars.inc)"
        e="чисто"; [ -s "$work/r/$n/erc-summary.txt" ] && e="⚠ есть замечания"
      fi
      if [ -f "$work/r/$n/code/report.md" ]; then
        code="[отчёт]($n/code/report.md)"; grep -q "^## ❌" "$work/r/$n/code/report.md" && code="❌ $code"
      fi
      echo "| ${n%%/*} | ${n#*/} | $sch | $pe | $par | $asm | $e | $code |"
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
  n="${d#build/}"
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
