// Package archtest guarda os testes de arquitetura (regras de dependência entre
// camadas). São testes de fronteira, não de comportamento: garantem que o
// domínio permanece a autoridade e não vaza acoplamento ao framework.
package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

const modulePath = "github.com/RafaelFino/psicoman"

// deps devolve a lista de dependências transitivas (imports) de um pacote do
// módulo, via `go list -deps`. Falha o teste se o comando não rodar.
func deps(t *testing.T, pkg string) []string {
	t.Helper()
	// -deps lista o pacote e todas as suas dependências (transitivas).
	out, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}

// TestDomainNaoImportaPocketBase garante a regra de dependência central (R5/D5,
// psicoman-golang.md): o domínio é a autoridade e NÃO importa o PocketBase nem o
// adapter de persistência platform/pb. Trocar o framework = mexer só no adapter.
func TestDomainNaoImportaPocketBase(t *testing.T) {
	proibidos := []string{
		"github.com/pocketbase/pocketbase",
		modulePath + "/internal/platform/pb",
		modulePath + "/internal/app",
		modulePath + "/internal/api",
	}

	for _, dep := range deps(t, modulePath+"/internal/domain/...") {
		for _, proibido := range proibidos {
			if dep == proibido || strings.HasPrefix(dep, proibido+"/") {
				t.Errorf("internal/domain não pode importar %q (dependência proibida encontrada: %q)", proibido, dep)
			}
		}
	}
}
