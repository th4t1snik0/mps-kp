#!/usr/bin/env bash
# Рендер схемы из папки сборки: PDF, PNG 300 dpi и отчёт ERC.
# Использование: scripts/render.sh build/<ник>
set -euo pipefail
d="$1"
[ -f "$d/schematic.kicad_sch" ] || { echo "нет $d/schematic.kicad_sch"; exit 0; }
ws=()
[ -f "$d/ramka.kicad_wks" ] && ws=(--drawing-sheet "$d/ramka.kicad_wks")
kicad-cli sch export pdf -b "${ws[@]}" -o "$d/schematic.pdf" "$d/schematic.kicad_sch"
pdftoppm -png -r 300 -singlefile "$d/schematic.pdf" "$d/schematic"
rm -f "$d"/~*.lck "$d/erc.rpt"
kicad-cli sch erc -o "$d/erc.rpt" "$d/schematic.kicad_sch" || true
grep -o "^\[[a-z_]*\]" "$d/erc.rpt" | sort | uniq -c > "$d/erc-summary.txt" || true
echo "готово: $d/schematic.pdf, $d/schematic.png, $d/erc-summary.txt"
