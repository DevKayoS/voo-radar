// Package alert contém a regra pura que decide quando vale a pena avisar.
package alert

import "github.com/DevKayoS/voo-radar/internal/domain/flight"

// Resultado da avaliação de alerta para a oferta mais barata atual.
type Resultado struct {
	DeveAvisar      bool
	MinimaHistorica bool   // o preço atual é a menor mínima já vista
	AbaixoDoAlvo    bool   // o preço atual <= preço alvo configurado
	Motivo          string // texto curto p/ a mensagem (ex.: "menor preço já visto")
}

// Avaliar decide se a oferta atual merece alerta, considerando o histórico
// (registros ANTERIORES, sem o atual) e o último preço já alertado.
//
// Regras (qualquer uma dispara):
//   - mínima histórica: preço < menor já registrado (exige histórico prévio);
//   - abaixo do alvo: preço <= alvo E melhorou desde o último alerta (anti-spam).
func Avaliar(atual flight.Offer, alvoCentavos int64, historico []flight.RegistroPreco, ultimoAlerta int64, temUltimo bool) Resultado {
	preco := atual.PrecoCentavos

	menor, temMenor := menorHistorico(historico)
	minimaHistorica := temMenor && preco < menor

	abaixoDoAlvo := alvoCentavos > 0 && preco <= alvoCentavos
	melhorouDesdeAlerta := !temUltimo || preco < ultimoAlerta

	deveAvisar := minimaHistorica || (abaixoDoAlvo && melhorouDesdeAlerta)

	motivo := ""
	switch {
	case minimaHistorica:
		motivo = "📉 menor preço já registrado"
	case abaixoDoAlvo:
		motivo = "🎯 abaixo do seu preço-alvo"
	}

	return Resultado{
		DeveAvisar:      deveAvisar,
		MinimaHistorica: minimaHistorica,
		AbaixoDoAlvo:    abaixoDoAlvo,
		Motivo:          motivo,
	}
}

// menorHistorico devolve o menor preço já registrado e se há registros.
func menorHistorico(historico []flight.RegistroPreco) (int64, bool) {
	if len(historico) == 0 {
		return 0, false
	}
	menor := historico[0].PrecoCentavos
	for _, r := range historico[1:] {
		if r.PrecoCentavos < menor {
			menor = r.PrecoCentavos
		}
	}
	return menor, true
}
