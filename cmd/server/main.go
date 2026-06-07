package main

import (
	"path/filepath"

	"github.com/fino/psicoman/internal/service"
	"github.com/fino/psicoman/internal/web"
)

func main() {
	cfg := web.LoadConfig()
	log := web.NewLogger(cfg.DataDir)

	var google *service.GoogleCalendar
	if cfg.GoogleClientID != "" {
		google = service.NewGoogleCalendar(
			cfg.GoogleClientID, cfg.GoogleClientSecret,
			cfg.GooglePsychRedirectURL, cfg.GoogleCalendarID,
		)
	}

	calendar := &service.DBCalendar{Google: google, Noop: &service.NoopCalendar{}}

	app := &web.App{
		Config:  cfg,
		Log:     log,
		Auth:    &service.AuthService{JWTSecret: cfg.JWTSecret, GoogleClientID: cfg.GoogleClientID, GoogleClientSecret: cfg.GoogleClientSecret, GoogleRedirectURL: cfg.GoogleRedirectURL},
		Patient: &service.PatientService{},
		Appt:    &service.AppointmentService{Calendar: calendar},
		GED:     &service.GEDService{BaseDir: filepath.Join(cfg.DataDir, "ged")},
		Finance: &service.FinanceService{},
		Google:  google,
	}

	log.Info().Str("addr", cfg.Addr).Msg("starting psicoman")
	if err := app.Router().Run(cfg.Addr); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
