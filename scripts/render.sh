#!/usr/bin/env bash
# Рендер папки сборки: схема (PDF, PNG 300 dpi, ERC) и перечень элементов (PDF, PNG постранично).
# Использование: scripts/render.sh build/<группа>/<вариант>
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

# перечень: perechen-N.kicad_sch + perechen-N.kicad_wks → perechen.pdf; то же для perechen-km1
for set in perechen perechen-km1; do
  pdfs=()
  for sch in "$d/$set"-[0-9]*.kicad_sch; do
    [ -f "$sch" ] || continue
    base="${sch%.kicad_sch}"
    kicad-cli sch export pdf -b --drawing-sheet "$base.kicad_wks" -o "$base.pdf" "$sch" >/dev/null
    pdftoppm -png -r 200 -singlefile "$base.pdf" "$base"
    pdfs+=("$base.pdf")
  done
  if [ ${#pdfs[@]} -gt 0 ]; then
    if [ ${#pdfs[@]} -gt 1 ]; then pdfunite "${pdfs[@]}" "$d/$set.pdf"; else cp "${pdfs[0]}" "$d/$set.pdf"; fi
  fi
done
echo "готово: $d/schematic.pdf, $d/schematic.png, $d/erc-summary.txt, $d/perechen.pdf, $d/perechen-km1.pdf"
