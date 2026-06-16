package utils

import (
	"fmt"
	"time"
)

var mesesAbrev = [...]string{
	"jan", "fev", "mar", "abr", "mai", "jun",
	"jul", "ago", "set", "out", "nov", "dez",
}

// FormatDiaMes formata "2026-10-12" como "12/out".
func FormatDiaMes(isoDate string) string {
	t, err := time.Parse("2006-01-02", isoDate)
	if err != nil {
		return isoDate
	}
	return fmt.Sprintf("%02d/%s", t.Day(), mesesAbrev[t.Month()-1])
}

// FormatVistoEm formata um instante como "15/06 14:32" (horário local).
func FormatVistoEm(t time.Time) string {
	return t.Local().Format("02/01 15:04")
}

// FormatYYMMDD converte "2026-10-12" em "261012" (formato de URL do Skyscanner).
func FormatYYMMDD(isoDate string) string {
	t, err := time.Parse("2006-01-02", isoDate)
	if err != nil {
		return isoDate
	}
	return t.Format("060102")
}
