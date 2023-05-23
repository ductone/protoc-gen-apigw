package apigw

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"text/template"
)

var (
	//go:embed templates/*.tmpl templates/layouts/*.tmpl
	templateFiles embed.FS
	templates     map[string]*template.Template
)

//nolint:gochecknoinits // compling templates from embed
func init() {
	err := loadTemplates()
	if err != nil {
		panic(fmt.Errorf("pgdb.loadTemplates failed; %w", err))
	}
}

func templateExecToString(name string, c interface{}) (string, error) {
	buf := bytes.Buffer{}
	err := templates[name].Execute(&buf, c)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func loadTemplates() error {
	templates = make(map[string]*template.Template)
	tmplFiles, err := fs.ReadDir(templateFiles, "templates")
	if err != nil {
		return err
	}

	for _, tmpl := range tmplFiles {
		if tmpl.IsDir() {
			continue
		}

		pt, err := template.ParseFS(templateFiles, path.Join("templates", tmpl.Name()), "templates/layouts/*.tmpl")
		if err != nil {
			return err
		}

		templates[tmpl.Name()] = pt
	}
	return nil
}

type varNamer struct {
	prefix string
	offset int
}

func (fn *varNamer) Next() *varNamer {
	return &varNamer{offset: fn.offset + 1, prefix: fn.prefix}
}

func (fn *varNamer) String() string {
	return fmt.Sprintf("%s%d", fn.prefix, fn.offset)
}
