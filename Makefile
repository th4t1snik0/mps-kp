# make try G=А-17 M=16 FIO="Иванов И.И." [CHK="Михалин С.Н."] [STYLE=A|B|C|D] — любой вариант → build/А-17-23/16/
# make student S=<ник>                      — из students/<ник>/variant.yaml → build/<группа>/<вариант>/
# make test
# make code S=<ник>                         — проверить students/<ник>/code/prog{1,2,3}.a51 → build/<группа>/<вариант>/code/
# make code-init S=<ник>                    — заготовки программ в students/<ник>/code/ (существующие не трогает)
# make code-try G=А-12 M=14 F=prog1.a51     — проверить любой файл под любой вариант
# Y — год набора группы (А-12 → А-12-$(Y)).
TABLE ?= data/table-2026.yaml
Y     ?= 23
FIO   ?=
CHK   ?= Михалин С.Н.
STYLE ?=

bin/mpsgen: $(shell find cmd internal -name '*.go')
	go build -o bin/mpsgen ./cmd/mpsgen

bin/mpscode: $(shell find cmd internal -name '*.go' -o -name '*.a51')
	go build -o bin/mpscode ./cmd/mpscode

test:
	go test ./...

student: bin/mpsgen
	./bin/mpsgen -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -style "$(STYLE)" -render

try: bin/mpsgen
	./bin/mpsgen -table $(TABLE) -year $(Y) -group "$(G)" -m $(M) -name "$(FIO)" -checker "$(CHK)" -style "$(STYLE)" -render

code: bin/mpscode
	./bin/mpscode -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml

code-init: bin/mpscode
	./bin/mpscode -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -init

code-try: bin/mpscode
	./bin/mpscode -table $(TABLE) -year $(Y) -group "$(G)" -m $(M) -name "$(FIO)" $(F)

# пересобрать masters/lib/mps.kicad_sym из библиотек KiCad (нужно только при правке cmd/mpslib)
KICAD_SYMBOLS ?= /Applications/KiCad/KiCad.app/Contents/SharedSupport/symbols
lib:
	go run ./cmd/mpslib -kicad $(KICAD_SYMBOLS)

.PHONY: test student try lib code code-init code-try
