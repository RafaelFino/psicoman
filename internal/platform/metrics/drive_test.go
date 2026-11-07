package metrics

import (
	"strings"
	"testing"
	"time"
)

func newDriveObs(clk *stepClock) *Obs { return New(Config{Clock: clk}) }

func TestDriveMetrics_BackupLocalEDrive(t *testing.T) {
	clk := &stepClock{t: time.Unix(1_700_000_000, 0)}
	o := newDriveObs(clk)
	d := o.Drive()

	d.Backup("local", true, 2*time.Second, 4096)
	d.Backup("drive", true, 5*time.Second, 5000)
	d.Backup("drive", false, time.Second, 0)

	out := scrape(t, o)
	for _, want := range []string{
		`psicoman_backup_total{destino="local",resultado="ok"} 1`,
		`psicoman_backup_total{destino="drive",resultado="ok"} 1`,
		`psicoman_backup_total{destino="drive",resultado="erro"} 1`,
		`psicoman_backup_duracao_segundos_count{destino="drive"} 2`,
		`psicoman_backup_tamanho_bytes{destino="local"} 4096`,
		`psicoman_backup_tamanho_bytes{destino="drive"} 5000`, // falha não sobrescreve
		`psicoman_backup_ultimo_sucesso_timestamp_segundos{destino="local"} 1.7e+09`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposição sem %q", want)
		}
	}
}

// TestDriveMetrics_TimestampComecaEmZero: antes do 1º sucesso o timestamp vale 0
// (documentado) — a série existe, então o alerta de "backup velho" funciona.
func TestDriveMetrics_TimestampComecaEmZero(t *testing.T) {
	out := scrape(t, newDriveObs(&stepClock{t: time.Unix(1_700_000_000, 0)}))
	for _, want := range []string{
		`psicoman_backup_ultimo_sucesso_timestamp_segundos{destino="local"} 0`,
		`psicoman_backup_ultimo_sucesso_timestamp_segundos{destino="drive"} 0`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposição sem %q", want)
		}
	}
}

func TestDriveMetrics_FalhaNaoAvancaOTimestamp(t *testing.T) {
	clk := &stepClock{t: time.Unix(1_700_000_000, 0)}
	o := newDriveObs(clk)
	o.Drive().Backup("drive", false, time.Second, 0)
	if !strings.Contains(scrape(t, o), `psicoman_backup_ultimo_sucesso_timestamp_segundos{destino="drive"} 0`) {
		t.Error("um backup com erro não pode atualizar o último sucesso")
	}
}

func TestDriveMetrics_EspelhamentoEDrPronto(t *testing.T) {
	o := newDriveObs(&stepClock{t: time.Unix(1_700_000_000, 0)})
	d := o.Drive()

	if !strings.Contains(scrape(t, o), "psicoman_drive_dr_pronto 0") {
		t.Error("drive_dr_pronto deveria começar em 0")
	}
	d.Mirror("ok")
	out := scrape(t, o)
	if !strings.Contains(out, "psicoman_drive_dr_pronto 1") ||
		!strings.Contains(out, `psicoman_drive_espelhamento_total{resultado="ok"} 1`) {
		t.Errorf("após Mirror ok: pronto=1 e contador ok=1 esperados:\n%s", out)
	}
	d.Mirror("erro")
	if !strings.Contains(scrape(t, o), "psicoman_drive_dr_pronto 0") {
		t.Error("após erro drive_dr_pronto deve voltar a 0")
	}
	d.Mirror("ok")
	d.Mirror("desativado")
	out = scrape(t, o)
	if !strings.Contains(out, "psicoman_drive_dr_pronto 0") ||
		!strings.Contains(out, `psicoman_drive_espelhamento_total{resultado="desativado"} 1`) {
		t.Errorf("desativado deve zerar o pronto e ser contado:\n%s", out)
	}
}

// TestDriveMetrics_SnapshotsMantidosAusenteQuandoListagemFalha (R8.6).
func TestDriveMetrics_SnapshotsMantidosAusenteQuandoListagemFalha(t *testing.T) {
	o := newDriveObs(&stepClock{t: time.Unix(1_700_000_000, 0)})
	d := o.Drive()

	if strings.Contains(scrape(t, o), "psicoman_drive_snapshots_mantidos") {
		t.Error("sem retenção ainda, a série deve estar ausente")
	}
	d.Retention(14)
	if !strings.Contains(scrape(t, o), "psicoman_drive_snapshots_mantidos 14") {
		t.Error("após Retention(14) a série deve existir")
	}
	d.RetentionFailure("lixeira") // falha de Trash: a contagem confirmada permanece
	out := scrape(t, o)
	if !strings.Contains(out, "psicoman_drive_snapshots_mantidos 14") ||
		!strings.Contains(out, `psicoman_drive_retencao_falhas_total{etapa="lixeira"} 1`) {
		t.Errorf("falha de lixeira conta e mantém o gauge:\n%s", out)
	}
	d.RetentionFailure("listar") // listagem falhou: a série SOME
	out = scrape(t, o)
	if strings.Contains(out, "psicoman_drive_snapshots_mantidos") {
		t.Error("listagem com falha deve tornar drive_snapshots_mantidos ausente")
	}
	if !strings.Contains(out, `psicoman_drive_retencao_falhas_total{etapa="listar"} 1`) {
		t.Error("falha de listar deve ser contada")
	}
}

// TestDriveMetrics_RotulosForaDoEnumViramOutro: nunca texto livre em rótulo.
func TestDriveMetrics_RotulosForaDoEnumViramOutro(t *testing.T) {
	o := newDriveObs(&stepClock{t: time.Unix(1_700_000_000, 0)})
	d := o.Drive()
	d.Backup("/home/fino/segredo", true, time.Second, 1)
	d.Mirror("paciente@x.com")
	d.RetentionFailure("qualquer coisa")

	out := scrape(t, o)
	for _, ruim := range []string{"segredo", "paciente@x.com", "qualquer coisa"} {
		if strings.Contains(out, ruim) {
			t.Errorf("valor livre %q vazou para um rótulo", ruim)
		}
	}
	for _, want := range []string{`destino="outro"`, `resultado="outro"`, `etapa="outro"`} {
		if !strings.Contains(out, want) {
			t.Errorf("esperava o rótulo normalizado %s", want)
		}
	}
}
