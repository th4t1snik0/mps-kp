#!/usr/bin/env bash
# Сводка по студенту после make all: ERC схемы, итог проверки программ, незаполненные места ПЗ.
# Использование: scripts/summary.sh <ник> [год набора, 23]
set -uo pipefail
s="$1"; y="${2:-23}"
v="students/$s/variant.yaml"
g="$(sed -n 's/^group: *\([^ #]*\).*/\1/p' "$v")"
m="$(sed -n 's/^m: *\([0-9]*\).*/\1/p' "$v")"
d="build/$g-$y/$m"
echo "== $s: $d"
if [ -f "$d/erc-summary.txt" ]; then
  [ -s "$d/erc-summary.txt" ] && echo "КМ-1  схема: ⚠ ERC — см. $d/erc-summary.txt" || echo "КМ-1  схема: ERC чисто ($d/schematic.png, perechen.pdf)"
fi
grep -h '^## ' "$d/code/report.md" 2>/dev/null | sed 's/^## /КМ-3  /'
for p in pz1 pz2; do
  r="$d/$p/report.md"
  lbl=ПЗ1; [ "$p" = pz2 ] && lbl=ПЗ2
  [ -f "$r" ] && echo "$lbl   $(grep -o 'осталось заполнить: [0-9]*' "$r") — $(ls "$d/$p/"*.docx 2>/dev/null | head -1)"
done
