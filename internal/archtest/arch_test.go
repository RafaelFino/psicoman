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

// TestAppNaoImportaPocketBase garante que a camada de aplicação (internal/app)
// orquestra o domínio pelos ports e NÃO conhece o framework (PocketBase) nem o
// adapter de persistência platform/pb. internal/app PODE importar platform/clock,
// platform/ulid e platform/crypto (ports/helpers) — só os dois abaixo são
// proibidos (R5/D5, psicoman-golang.md; enunciado da Fase 4, M1).
func TestAppNaoImportaPocketBase(t *testing.T) {
	proibidos := []string{
		"github.com/pocketbase/pocketbase",
		modulePath + "/internal/platform/pb",
	}

	for _, dep := range deps(t, modulePath+"/internal/app/...") {
		for _, proibido := range proibidos {
			if dep == proibido || strings.HasPrefix(dep, proibido+"/") {
				t.Errorf("internal/app não pode importar %q (dependência proibida encontrada: %q)", proibido, dep)
			}
		}
	}
}
