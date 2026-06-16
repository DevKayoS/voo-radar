package alert

import (
	"testing"

	"github.com/DevKayoS/voo-radar/internal/domain/flight"
)

func oferta(centavos int64) flight.Offer {
	return flight.Offer{PrecoCentavos: centavos}
}

func historicoCom(precos ...int64) []flight.RegistroPreco {
	regs := make([]flight.RegistroPreco, len(precos))
	for i, p := range precos {
		regs[i] = flight.RegistroPreco{PrecoCentavos: p}
	}
	return regs
}

func TestAvaliar(t *testing.T) {
	const alvo int64 = 250000 // R$2.500

	casos := []struct {
		nome         string
		preco        int64
		historico    []flight.RegistroPreco
		ultimoAlerta int64
		temUltimo    bool
		querAvisar   bool
		querMinima   bool
	}{
		{
			nome:       "primeira coleta acima do alvo: não avisa",
			preco:      300000,
			querAvisar: false,
		},
		{
			nome:       "primeira coleta abaixo do alvo: avisa",
			preco:      240000,
			querAvisar: true,
		},
		{
			nome:       "nova mínima histórica: avisa",
			preco:      260000,
			historico:  historicoCom(280000, 270000),
			querAvisar: true,
			querMinima: true,
		},
		{
			nome:       "acima da mínima e acima do alvo: não avisa",
			preco:      275000,
			historico:  historicoCom(270000, 290000),
			querAvisar: false,
		},
		{
			nome:         "abaixo do alvo mas não melhorou desde o último alerta: não avisa",
			preco:        240000,
			historico:    historicoCom(240000),
			ultimoAlerta: 240000,
			temUltimo:    true,
			querAvisar:   false,
		},
		{
			nome:         "abaixo do alvo e melhorou desde o último alerta: avisa",
			preco:        230000,
			historico:    historicoCom(240000),
			ultimoAlerta: 240000,
			temUltimo:    true,
			querAvisar:   true,
			querMinima:   true, // 230000 < menor histórico (240000)
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			res := Avaliar(oferta(c.preco), alvo, c.historico, c.ultimoAlerta, c.temUltimo)
			if res.DeveAvisar != c.querAvisar {
				t.Errorf("DeveAvisar = %v, queria %v", res.DeveAvisar, c.querAvisar)
			}
			if res.MinimaHistorica != c.querMinima {
				t.Errorf("MinimaHistorica = %v, queria %v", res.MinimaHistorica, c.querMinima)
			}
		})
	}
}
