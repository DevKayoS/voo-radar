package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// ToReais converte centavos para reais.
func ToReais(centavos int64) float64 {
	return float64(centavos) / 100.0
}

// ToCentavos converte reais para centavos, arredondando.
func ToCentavos(reais float64) int64 {
	if reais < 0 {
		return int64(reais*100 - 0.5)
	}
	return int64(reais*100 + 0.5)
}

// FormatBRL formata centavos como "R$ 2.180,00".
func FormatBRL(centavos int64) string {
	neg := centavos < 0
	if neg {
		centavos = -centavos
	}
	reais := centavos / 100
	cent := centavos % 100
	sinal := ""
	if neg {
		sinal = "-"
	}
	return fmt.Sprintf("R$ %s%s,%02d", sinal, agruparMilhares(reais), cent)
}

// agruparMilhares insere pontos a cada 3 dígitos: 2180 -> "2.180".
func agruparMilhares(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		b.WriteByte('.')
	}
	for i := pre; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte('.')
		}
	}
	return b.String()
}
