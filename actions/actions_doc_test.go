// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package actions_test

import (
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/parser"
	"go/token"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseActionsPackage parses the non-test sources of the actions package
// and returns its files and the rendered package documentation.
func parseActionsPackage(t *testing.T) (map[string]*ast.File, *doc.Package) {
	t.Helper()

	fset := token.NewFileSet()
	notTest := func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}
	pkgs, err := parser.ParseDir(fset, ".", notTest, parser.ParseComments)
	require.NoError(t, err)

	pkg, ok := pkgs["actions"]
	require.True(t, ok, "actions package not found")

	files := make([]*ast.File, 0, len(pkg.Files))
	for _, f := range pkg.Files {
		files = append(files, f)
	}

	d, err := doc.NewFromFiles(fset, files, "github.com/go-debos/debos/actions", doc.PreserveAST)
	require.NoError(t, err)

	return pkg.Files, d
}

// registeredActions returns the action names handled by
// YamlAction.UnmarshalYAML in recipe.go.
func registeredActions(t *testing.T, files map[string]*ast.File) []string {
	t.Helper()

	var names []string
	for path, f := range files {
		if !strings.HasSuffix(path, "recipe.go") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "UnmarshalYAML" || fn.Recv == nil {
				return true
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				cc, ok := n.(*ast.CaseClause)
				if !ok {
					return true
				}
				for _, e := range cc.List {
					lit, ok := e.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					name, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					names = append(names, name)
				}

				return true
			})

			return false
		})
	}
	require.NotEmpty(t, names, "no actions found in YamlAction.UnmarshalYAML")

	return names
}

// TestActionsDoc checks that every action is listed in the package
// documentation with a link to a heading which is actually rendered,
// e.g. on pkg.go.dev.
func TestActionsDoc(t *testing.T) {
	files, d := parseActionsPackage(t)

	var p comment.Parser
	parsed := p.Parse(d.Doc)

	// Collect the anchors of every heading in the rendered documentation.
	var printer comment.Printer
	html := string(printer.HTML(parsed))
	headings := map[string]bool{}
	for _, m := range regexp.MustCompile(`<h[1-6] id="(hdr-[^"]*)"`).FindAllStringSubmatch(html, -1) {
		headings[m[1]] = true
	}

	// Collect the "- name -- https://...#hdr-Anchor" entries listed under
	// "Supported actions".
	links := map[string]string{}
	re := regexp.MustCompile(`(?m)^- ([a-z-]+) -- \S+#(hdr-\S+)$`)
	for _, m := range re.FindAllStringSubmatch(d.Doc, -1) {
		links[m[1]] = m[2]
	}

	for _, name := range registeredActions(t, files) {
		anchor, ok := links[name]
		if !assert.True(t, ok, "action %q is not listed under \"Supported actions\"", name) {
			continue
		}
		assert.True(t, headings[anchor],
			"action %q links to #%s but no such heading is rendered; check the "+
				"action's comment directly precedes the package clause and its "+
				"heading starts with a capital letter", name, anchor)
	}
}
