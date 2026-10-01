package variant

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Meta — с какими данными собрана схема (meta.json рядом со schematic.kicad_sch). По нему КМ-2 и КМ-3 проверяют,
// что берут схему того же студента и что variant.yaml с тех пор не менялся; дата из него идёт на титул ПЗ1.
type Meta struct {
	Group     string `json:"group"`
	GroupFull string `json:"group_full"`
	M         int    `json:"m"`
	Student   string `json:"student"`
	Checker   string `json:"checker"`
	StyleReq  string `json:"style_req"`       // как задано в variant.yaml / форме ("" — авто)
	Style     string `json:"style"`           // какой вышел
	Date      string `json:"date"`            // дата в рамке
	Fixes     string `json:"fixes,omitempty"` // отпечаток schema/fixes.yaml (правки по замечаниям); пусто — нет
	Nick      string `json:"nick,omitempty"`
	Commit    string `json:"commit,omitempty"`
	Dest      string `json:"dest"` // папка в results и на диске: «А-12-23/17 Рязанцев И.В.»
}

// Dest — папка студента в ветке results и на Яндекс-диске: «<группа>/<M> <Фамилия И.О.>».
func (s *Student) Dest() string {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		name = "без ФИО"
	}
	return fmt.Sprintf("%s/%d %s", s.GroupFull, s.M, name)
}

// Meta — метаданные схемы студента (style — вышедший стиль).
func (s *Student) Meta(style string) Meta {
	m := Meta{Group: s.Group, GroupFull: s.GroupFull, M: s.M, Student: s.Name, Checker: s.Checker,
		StyleReq: strings.TrimSpace(s.Style), Style: style, Date: s.Date, Commit: os.Getenv("GITHUB_SHA"), Dest: s.Dest()}
	if s.Path != "" {
		m.Nick = filepath.Base(filepath.Dir(s.Path))
		if b, err := os.ReadFile(s.FixesPath()); err == nil {
			m.Fixes = fmt.Sprintf("%x", sha256.Sum256(b))[:12]
		}
	}
	return m
}

// FixesPath — students/<ник>/schema/fixes.yaml (правки схемы по замечаниям); "" — студент не из папки.
func (s *Student) FixesPath() string {
	if s.Path == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.Path), "schema", "fixes.yaml")
}

// LoadMeta читает meta.json.
func LoadMeta(path string) (Meta, error) {
	var m Meta
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(b, &m)
}

// Stale — чем схема (m) расходится с текущим variant.yaml (cur); пусто — не расходится.
func (m Meta) Stale(cur Meta) []string {
	var d []string
	cmp := func(what, a, b string) {
		if a != b {
			d = append(d, fmt.Sprintf("%s: в схеме «%s», в variant.yaml «%s»", what, a, b))
		}
	}
	cmp("группа", m.GroupFull, cur.GroupFull)
	cmp("вариант", fmt.Sprint(m.M), fmt.Sprint(cur.M))
	cmp("ФИО", m.Student, cur.Student)
	cmp("проверяющий", m.Checker, cur.Checker)
	cmp("стиль", m.StyleReq, cur.StyleReq)
	if m.Fixes != cur.Fixes {
		d = append(d, "правки схемы (schema/fixes.yaml) менялись после сборки")
	}
	return d
}
