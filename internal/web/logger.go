package web

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(dataDir string) zerolog.Logger {
	logDir := filepath.Join(dataDir, "logs")
	_ = os.MkdirAll(logDir, 0755)

	writer := io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "psicoman-"+time.Now().Format("2006-01-02")+".json"),
		MaxSize:    100,
		MaxBackups: 30,
		MaxAge:     90,
		Compress:   true,
	})

	return zerolog.New(writer).With().Timestamp().Logger()
}
