package service

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// ReportRepository fornece agregações para relatórios financeiros, de custos e ROI.
type ReportRepository interface {
	// RevenueByOrigin soma o valor recebido (pagamentos) por origem no período.
	RevenueByOrigin(ctx context.Context, de, ate time.Time) (map[string]int64, error)
	// DebtsSummary agrega débitos gerados, em aberto e recebidos no período.
	DebtsSummary(ctx context.Context, de, ate time.Time) (*FinancialSummary, error)
	// CostTotal soma os custos periodizados no intervalo (proporcional ao período).
	CostTotalByKind(ctx context.Context, de, ate time.Time) (map[string]int64, error)
	// SessionCostByPatient soma o custo atribuído às sessões de cada paciente.
	SessionCostByPatient(ctx context.Context, de, ate time.Time) (map[string]int64, error)
}

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

// ReportService produz relatórios financeiros, de custos e ROI por canal.
type ReportService struct {
	reports ReportRepository
	origins OriginRepository
	costs   CostRepository
}

// NewReportService cria o serviço de relatórios.
func NewReportService(reports ReportRepository, origins OriginRepository, costs CostRepository) *ReportService {
	return &ReportService{reports: reports, origins: origins, costs: costs}
}

// Financial devolve o resumo financeiro do período.
func (s *ReportService) Financial(ctx context.Context, de, ate time.Time) (*FinancialSummary, error) {
	return s.reports.DebtsSummary(ctx, de, ate)
}

// CostReport agrega custos por categoria e por paciente no período.
type CostReport struct {
	ByKind    map[string]int64 `json:"by_kind"`
	ByPatient map[string]int64 `json:"by_patient"`
}

// Costs devolve o relatório de custos do período.
func (s *ReportService) Costs(ctx context.Context, de, ate time.Time) (*CostReport, error) {
	byKind, err := s.reports.CostTotalByKind(ctx, de, ate)
	if err != nil {
		return nil, err
	}
	byPatient, err := s.reports.SessionCostByPatient(ctx, de, ate)
	if err != nil {
		return nil, err
	}
	return &CostReport{ByKind: byKind, ByPatient: byPatient}, nil
}

// ROI cruza receita gerada pelos pacientes de cada origem com o custo da
// plataforma correspondente no período (requirements §3.5).
func (s *ReportService) ROI(ctx context.Context, de, ate time.Time) ([]ROIRow, error) {
	revenue, err := s.reports.RevenueByOrigin(ctx, de, ate)
	if err != nil {
		return nil, err
	}
	origins, err := s.origins.List(ctx)
	if err != nil {
		return nil, err
	}

	// Custo por origem = soma dos itens de plataforma ligados àquela origem,
	// periodizados para o intervalo.
	days := ate.Sub(de).Hours() / 24
	if days < 1 {
		days = 1
	}

	var rows []ROIRow
	for _, o := range origins {
		items, err := s.costs.ListItemsByOrigin(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		var cost int64
		for _, it := range items {
			cost += periodizedCost(it, days)
		}
		rev := revenue[o.ID]
		rows = append(rows, ROIRow{
			OriginID:   o.ID,
			OriginName: o.Name,
			Revenue:    rev,
			Cost:       cost,
			ROI:        rev - cost,
		})
	}
	return rows, nil
}

// periodizedCost converte o custo do item para o intervalo de `days` dias.
func periodizedCost(it *domain.CostItem, days float64) int64 {
	switch it.Period {
	case domain.PeriodDiario:
		return int64(float64(it.Amount) * days)
	case domain.PeriodMensal:
		return int64(float64(it.Amount) * days / 30.0)
	case domain.PeriodAnual:
		return int64(float64(it.Amount) * days / 365.0)
	default: // por_sessao não periodiza de forma temporal
		return it.Amount
	}
}
