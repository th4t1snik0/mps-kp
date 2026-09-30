#!/usr/bin/env bash
# Сверка нашего ассемблера (asm51) с MCU 8051 IDE: каждая build*/<группа>/<M>/code/progN.a51 собирается обоими
# с тем же vars.inc, байты должны совпасть. Нужны: mcu8051ide (apt), bin/mpscode.
# Использование: scripts/mide_crosscheck.sh build build-ref
set -uo pipefail
mide="${MIDE:-mcu8051ide}"
work="$(mktemp -d)"
n=0 bad=0
while IFS= read -r -d '' f; do
  d="$work/$n"; n=$((n + 1))
  mkdir -p "$d"
  cp "$f" "$d/p.a51"
  cp "$(dirname "$f")/vars.inc" "$d/vars.inc"
  sed -i '1s/^\xEF\xBB\xBF//' "$d/p.a51"   # BOM MIDE не понимает (в файл для робота он и так не попадает)
  if ! (cd "$d" && "$mide" -n --compile p.a51 >log.txt 2>&1) || [ ! -s "$d/p.hex" ]; then
    echo "⚠ $f: MCU 8051 IDE не собрал — сверить нечем:"
    sed 's/^/    /' "$d/log.txt" | tail -15
    continue
  fi
  bin/mpscode -asm "$d/p.a51" -vars "$d/vars.inc" -cmp "$d/p.hex" 2>&1 | sed "s|$d/p.a51|$f|; s|$d/p.hex|MIDE|" || bad=$((bad + 1))
done < <(find "$@" -path '*/code/prog[123].a51' -print0 2>/dev/null)
echo "сверено файлов: $n, расхождений: $bad"
[ "$bad" -eq 0 ]
