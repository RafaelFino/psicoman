package app

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/costs"
)

// ReportService produz relatórios financeiros, de custos e ROI por canal. A
// periodização de custo vive no domínio costs; o serviço coordena as fontes de
// agregação. Portado de c3ab585:internal/service/report.go.
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
func (s *ReportService) Financial(ctx context.Context, de, ate time.Time) (*costs.FinancialSummary, error) {
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
func (s *ReportService) ROI(ctx context.Context, de, ate time.Time) ([]costs.ROIRow, error) {
	revenue, err := s.reports.RevenueByOrigin(ctx, de, ate)
	if err != nil {
		return nil, err
	}
	origins, err := s.origins.List(ctx)
	if err != nil {
		return nil, err
	}
	// Custo por origem = soma dos itens de plataforma ligados àquela origem,
	// periodizados para o intervalo. days é só fator temporal; o dinheiro
	// permanece int64 centavos.
	days := ate.Sub(de).Hours() / 24
	if days < 1 {
		days = 1
	}
	var rows []costs.ROIRow
	for _, o := range origins {
		items, err := s.costs.ListItemsByOrigin(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		var cost int64
		for _, it := range items {
			cost += costs.PeriodizedCost(it, days)
		}
		rev := revenue[o.ID]
		rows = append(rows, costs.ROIRow{
			OriginID:   o.ID,
			OriginName: o.Name,
			Revenue:    rev,
			Cost:       cost,
			ROI:        rev - cost,
		})
	}
	return rows, nil
}
