package portal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parsePortalFiles parseia os .go não-teste do diretório atual e devolve suas
// ASTs. Substitui go/parser.ParseDir (deprecado desde o Go 1.25) por ParseFile
// por arquivo, sem trazer dependência externa (go/packages). `mode` segue a
// semântica do parser (parser.ImportsOnly para imports, 0 para a AST inteira).
func parsePortalFiles(t *testing.T, fset *token.FileSet, mode parser.Mode) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("listar o pacote portal: %v", err)
	}
	files := make(map[string]*ast.File)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Clean(name), nil, mode)
		if err != nil {
			t.Fatalf("parse de %s: %v", name, err)
		}
		files[name] = f
	}
	return files
}

// TestPortalNaoImportaClinico garante, por análise estática dos .go do próprio
// pacote (go/parser), que o ator portal NÃO importa diretamente o bounded
// context clínico — por construção, nenhum DTO do portal pode carregar campo
// clínico (D6.3/R6.3). Testes são ignorados (não contam como superfície pública).
func TestPortalNaoImportaClinico(t *testing.T) {
	fset := token.NewFileSet()
	files := parsePortalFiles(t, fset, parser.ImportsOnly)
	for name, file := range files {
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(path, "internal/domain/clinical") {
				t.Errorf("%s importa o bounded context clínico (%q) — proibido no portal", name, path)
			}
		}
	}
}

// TestPortalNaoRevelaClinico garante que o código do pacote portal nunca chama
// SealedText.Reveal() (o único caminho para o claro clínico). Varre a AST
// (comentários não entram), afirmando a ausência da seleção ".Reveal".
func TestPortalNaoRevelaClinico(t *testing.T) {
	fset := token.NewFileSet()
	files := parsePortalFiles(t, fset, 0)
	for name, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && sel.Sel != nil && sel.Sel.Name == "Reveal" {
				t.Errorf("%s referencia .Reveal() — o portal nunca revela clínico", name)
			}
			return true
		})
	}
}
