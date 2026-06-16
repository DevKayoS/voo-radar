package collect

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevKayoS/voo-radar/internal/domain/flight"
	"github.com/DevKayoS/voo-radar/internal/usecases/alert"
	"github.com/DevKayoS/voo-radar/internal/utils"
)

// nomesCia traduz códigos IATA comuns no Brasil; cai no próprio código se não conhecer.
var nomesCia = map[string]string{
	"LA": "LATAM",
	"JJ": "LATAM",
	"G3": "GOL",
	"AD": "Azul",
	"AR": "Aerolíneas Argentinas",
	"H2": "Sky Airline",
	"AV": "Avianca",
	"CM": "Copa",
	"AA": "American",
	"UX": "Air Europa",
}

// formatarMensagem monta o texto Markdown enviado ao Telegram.
func formatarMensagem(o flight.Offer, b flight.Busca, res alert.Resultado, agora time.Time) string {
	var sb strings.Builder

	sb.WriteString("✈️ *Preço bom achado!*\n\n")
	sb.WriteString(fmt.Sprintf("%s → %s  (ida e volta)\n", o.Origem, o.Destino))
	sb.WriteString(fmt.Sprintf("📅 %s → %s\n", utils.FormatDiaMes(o.Ida), utils.FormatDiaMes(o.Volta)))

	linhaPreco := fmt.Sprintf("💰 *%s*", utils.FormatBRL(o.PrecoCentavos))
	if b.Filtros.PrecoAlvoCentavos > 0 {
		linhaPreco += fmt.Sprintf("  (alvo: %s)", utils.FormatBRL(b.Filtros.PrecoAlvoCentavos))
	}
	sb.WriteString(linhaPreco + "\n")

	if res.Motivo != "" {
		sb.WriteString(res.Motivo + "\n")
	}

	sb.WriteString(fmt.Sprintf("\nCia: %s · %s\n", nomeCia(o.Companhia), textoParadas(o.Paradas)))
	sb.WriteString("Visto em: " + utils.FormatVistoEm(agora))

	return sb.String()
}

func nomeCia(code string) string {
	if nome, ok := nomesCia[code]; ok {
		return nome
	}
	if code == "" {
		return "?"
	}
	return code
}

func textoParadas(n int) string {
	switch {
	case n <= 0:
		return "direto"
	case n == 1:
		return "1 parada"
	default:
		return fmt.Sprintf("%d paradas", n)
	}
}
