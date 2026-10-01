#!/usr/bin/env bash
# Ники студентов, у которых в этом пуше поменялись файлы по шаблонам (пути внутри students/<ник>/):
#   BEFORE=<sha до пуша> scripts/changed_students.sh variant.yaml 'code/*' pz/pz2.md 'pz/flow/*'
# Нужна история (actions/checkout с fetch-depth: 0). BEFORE пусто/нули (новая ветка) — сравнение с HEAD~1.
set -euo pipefail
base="${BEFORE:-}"
if [ -z "$base" ] || [ -z "${base//0/}" ] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  base="$(git rev-parse -q --verify HEAD~1 || true)"
fi
[ -n "$base" ] || exit 0
git -c core.quotePath=false diff --name-only "$base" HEAD -- students/ | while IFS= read -r f; do
  nick="$(echo "$f" | cut -d/ -f2)"
  rest="${f#students/$nick/}"
  [ -f "students/$nick/variant.yaml" ] || continue
  for pat in "$@"; do
    # shellcheck disable=SC2053
    [[ "$rest" == $pat ]] && { echo "$nick"; break; }
  done
done | sort -u
