# make student S=ivan        — собрать одного
# make try G=А-17 M=16        — прикинуть любой вариант без файла студента
# make test
TABLE ?= data/table-2025.yaml

bin/mpsgen: $(shell find cmd internal -name '*.go')
	go build -o bin/mpsgen ./cmd/mpsgen

test:
	go test ./...

student: bin/mpsgen
	./bin/mpsgen -table $(TABLE) -student students/$(S)/variant.yaml -out build/$(S)
	scripts/render.sh build/$(S)

try: bin/mpsgen
	./bin/mpsgen -table $(TABLE) -group $(G) -m $(M) -out build/try-$(G)-$(M)
	scripts/render.sh build/try-$(G)-$(M)

# пересобрать masters/lib/mps.kicad_sym из библиотек KiCad (нужно только при правке cmd/mpslib)
KICAD_SYMBOLS ?= /Applications/KiCad/KiCad.app/Contents/SharedSupport/symbols
lib:
	go run ./cmd/mpslib -kicad $(KICAD_SYMBOLS)

.PHONY: test student try lib
