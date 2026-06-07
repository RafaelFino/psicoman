package service

import (
	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type FinanceService struct{}

func (s *FinanceService) Summary(db *storage.DB, month, year int) (*domain.FinanceSummary, error) {
	payments, err := db.ListPayments(month, year)
	if err != nil {
		return nil, err
	}
	costs, err := db.ListCosts(month, year)
	if err != nil {
		return nil, err
	}

	summary := &domain.FinanceSummary{Month: month, Year: year, Payments: payments, Costs: costs}
	for _, p := range payments {
		if p.Status == domain.PaymentReceived {
			summary.TotalReceived += p.AmountCents
		} else {
			summary.TotalPending += p.AmountCents
		}
	}
	for _, c := range costs {
		summary.TotalCosts += c.AmountCents
	}
	summary.Balance = summary.TotalReceived - summary.TotalCosts
	return summary, nil
}

func (s *FinanceService) MonthlyReports(db *storage.DB, month, year int) ([]domain.MonthlyReport, error) {
	appts, err := db.ListCompletedInMonth(month, year)
	if err != nil {
		return nil, err
	}

	byPatient := map[string]*domain.MonthlyReport{}
	for _, a := range appts {
		if a.Status != domain.StatusCompleted && a.Status != domain.StatusScheduled {
			continue
		}
		r, ok := byPatient[a.PatientID]
		if !ok {
			r = &domain.MonthlyReport{
				Month: month, Year: year,
				PatientID: a.PatientID, PatientName: a.PatientName,
			}
			byPatient[a.PatientID] = r
		}
		r.Appointments = append(r.Appointments, a)
	}

	payments, _ := db.ListPayments(month, year)
	for _, p := range payments {
		if r, ok := byPatient[p.PatientID]; ok {
			r.TotalAmount += p.AmountCents
		}
	}

	var reports []domain.MonthlyReport
	for _, r := range byPatient {
		reports = append(reports, *r)
	}
	return reports, nil
}

func (s *FinanceService) AddPayment(db *storage.DB, p domain.Payment) (*domain.Payment, error) {
	if p.Status == "" {
		p.Status = domain.PaymentPending
	}
	return db.CreatePayment(p)
}

func (s *FinanceService) AddCost(db *storage.DB, c domain.Cost) (*domain.Cost, error) {
	return db.CreateCost(c)
}

func (s *FinanceService) ReceivePayment(db *storage.DB, id string) error {
	return db.ReceivePayment(id)
}
