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
	sb.WriteString(fmt.Sprintf("🔗 [Ver no Skyscanner](%s)\n", linkSkyscanner(o, b.Adultos)))
	sb.WriteString("Visto em: " + utils.FormatVistoEm(agora))

	return sb.String()
}

// linkSkyscanner monta a URL de busca do Skyscanner para a rota e datas exatas.
// Ex.: https://www.skyscanner.com.br/transporte/passagens-aereas/gru/scl/261012/261017/?adults=1
func linkSkyscanner(o flight.Offer, adultos int) string {
	if adultos < 1 {
		adultos = 1
	}
	return fmt.Sprintf(
		"https://www.skyscanner.com.br/transporte/passagens-aereas/%s/%s/%s/%s/?adults=%d",
		strings.ToLower(o.Origem), strings.ToLower(o.Destino),
		utils.FormatYYMMDD(o.Ida), utils.FormatYYMMDD(o.Volta), adultos,
	)
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
