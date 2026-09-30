# make try G=А-17 M=16 FIO="Иванов И.И." [CHK="Михалин С.Н."] — любой вариант → build/А-17-23/16/
# make student S=<ник>                      — из students/<ник>/variant.yaml → build/<группа>/<вариант>/
# make test
# Y — год набора группы (А-12 → А-12-$(Y)).
TABLE ?= data/table-2026.yaml
Y     ?= 23
FIO   ?=
CHK   ?= Михалин С.Н.

bin/mpsgen: $(shell find cmd internal -name '*.go')
	go build -o bin/mpsgen ./cmd/mpsgen

test:
	go test ./...

student: bin/mpsgen
	./bin/mpsgen -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -render

try: bin/mpsgen
	./bin/mpsgen -table $(TABLE) -year $(Y) -group "$(G)" -m $(M) -name "$(FIO)" -checker "$(CHK)" -render

# пересобрать masters/lib/mps.kicad_sym из библиотек KiCad (нужно только при правке cmd/mpslib)
KICAD_SYMBOLS ?= /Applications/KiCad/KiCad.app/Contents/SharedSupport/symbols
lib:
	go run ./cmd/mpslib -kicad $(KICAD_SYMBOLS)

.PHONY: test student try lib
