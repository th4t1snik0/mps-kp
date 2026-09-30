# make try G=А-17 M=16 FIO="Иванов И.И." [CHK="Михалин С.Н."] [STYLE=A|B|C|D] — любой вариант → build/А-17-23/16/
# make student S=<ник>                      — из students/<ник>/variant.yaml → build/<группа>/<вариант>/
# make test
# make code S=<ник>                         — проверить students/<ник>/code/prog{1,2,3}.a51 → build/<группа>/<вариант>/code/
# make code-init S=<ник>                    — заготовки программ в students/<ник>/code/ (существующие не трогает)
# make code-try G=А-12 M=14 F=prog1.a51     — проверить любой файл под любой вариант
# make all S=<ник>                          — всё, что делают джобы КМ-1…КМ-3, локально + сводка (нужны KiCad и pandoc)
# make pz1 S=<ник>                          — схема + ПЗ1 → build/<группа>/<вариант>/pz1/ (нужны KiCad и pandoc)
# make pz2-init S=<ник>                     — места «ДОПИШИ» (students/<ник>/pz/pz2.md) + заготовки схем алгоритмов
# make pz2 S=<ник>                          — ПЗ2 → build/<группа>/<вариант>/pz2/«Фамилия ИО ПЗ2.docx» (нужен pandoc)
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

bin/mpspz: $(shell find cmd internal -name '*.go' -o -name '*.flow')
	go build -o bin/mpspz ./cmd/mpspz

pz1: bin/mpspz student
	./bin/mpspz -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -doc pz1

pz1-init: bin/mpspz
	./bin/mpspz -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -doc pz1 -init

pz2: bin/mpspz
	./bin/mpspz -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -doc pz2

pz2-init: bin/mpspz
	./bin/mpspz -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -doc pz2 -init

all: student bin/mpscode bin/mpspz
	-./bin/mpscode -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml > /dev/null
	./bin/mpspz -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -doc pz1 > /dev/null
	./bin/mpspz -table $(TABLE) -year $(Y) -student students/$(S)/variant.yaml -doc pz2 > /dev/null
	@scripts/summary.sh $(S) $(Y)

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

.PHONY: all test student try lib code code-init code-try pz1 pz1-init pz2 pz2-init
