package portal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestPortalNaoImportaClinico garante, por análise estática dos .go do próprio
// pacote (go/parser), que o ator portal NÃO importa diretamente o bounded
// context clínico — por construção, nenhum DTO do portal pode carregar campo
// clínico (D6.3/R6.3). Testes são ignorados (não contam como superfície pública).
func TestPortalNaoImportaClinico(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse do pacote portal: %v", err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.Contains(path, "internal/domain/clinical") {
					t.Errorf("%s importa o bounded context clínico (%q) — proibido no portal", name, path)
				}
			}
		}
	}
}

// TestPortalNaoRevelaClinico garante que o código do pacote portal nunca chama
// SealedText.Reveal() (o único caminho para o claro clínico). Varre a AST
// (comentários não entram), afirmando a ausência da seleção ".Reveal".
func TestPortalNaoRevelaClinico(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse do pacote portal: %v", err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if ok && sel.Sel != nil && sel.Sel.Name == "Reveal" {
					t.Errorf("%s referencia .Reveal() — o portal nunca revela clínico", name)
				}
				return true
			})
		}
	}
}
