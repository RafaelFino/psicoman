package costs

import "github.com/RafaelFino/psicoman/internal/domain/scheduling"

// FinancialSummary agrega os números financeiros de um período.
type FinancialSummary struct {
	Generated int64 // total de débitos gerados (centavos)
	Open      int64 // total em aberto
	Received  int64 // total recebido (pagamentos)
	Overdue   int64 // total vencido e não quitado
}

// ROIRow é uma linha do relatório de ROI por canal.
type ROIRow struct {
	OriginID   string `json:"origin_id"`
	OriginName string `json:"origin_name"`
	Revenue    int64  `json:"revenue"`
	Cost       int64  `json:"cost"`
	ROI        int64  `json:"roi"` // revenue - cost (centavos)
}

// PeriodizedCost converte o custo do item para o intervalo de `days` dias. A
// periodização usa float64 APENAS como fator temporal (dias); o resultado é
// truncado de volta para int64 centavos — o dinheiro nunca deixa de ser inteiro.
// Portado de service/report.go:periodizedCost (paridade de arredondamento).
func PeriodizedCost(it *CostItem, days float64) int64 {
	switch it.Period {
	case scheduling.PeriodDiario:
		return int64(float64(it.Amount) * days)
	case scheduling.PeriodMensal:
		return int64(float64(it.Amount) * days / 30.0)
	case scheduling.PeriodAnual:
		return int64(float64(it.Amount) * days / 365.0)
	default: // por_sessao não periodiza de forma temporal
		return it.Amount
	}
}
