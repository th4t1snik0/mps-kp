package pz

import (
	"os"
	"testing"
)

// MPS_DOCX=путь.docx go test ./internal/pz -run TestCheckDocx — проверить готовый файл.
func TestCheckDocx(t *testing.T) {
	p := os.Getenv("MPS_DOCX")
	if p == "" {
		t.Skip("MPS_DOCX не задан")
	}
	errs, err := CheckOOXML(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range errs {
		t.Error(e)
	}
}
