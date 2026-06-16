package flight

import (
	"context"
	"slices"
	"time"
)

// Offer representa uma oferta de voo retornada por um OfferProvider.
type Offer struct {
	Origem        string
	Destino       string
	Ida           string // YYYY-MM-DD
	Volta         string // YYYY-MM-DD
	PrecoCentavos int64
	Moeda         string
	Companhia     string  // código IATA da cia validadora (ex.: "LA")
	Paradas       int     // maior número de paradas entre os trechos (ida/volta)
	DuracaoHoras  float64 // maior duração entre os trechos, em horas
	Fonte         string  // ex.: "skyscanner"
}

// Busca é uma combinação concreta de rota + datas a consultar num provider.
type Busca struct {
	Origem  string
	Destino string
	Ida     string
	Volta   string
	Adultos int
	Moeda   string
	Filtros Filtros
}

// Chave identifica unicamente uma combinação rota+datas, usada para
// histórico e estado de alerta.
func (b Busca) Chave() Chave {
	return Chave{Origem: b.Origem, Destino: b.Destino, Ida: b.Ida, Volta: b.Volta}
}

// Filtros de seleção de ofertas, lidos da config (nada hardcoded).
type Filtros struct {
	MaxParadas        int      // >=1 limita conexões; 0 = sem limite
	SomenteDireto     bool     // true = só voo direto
	CompanhiasIncluir []string // IATA; vazio = todas
	CompanhiasExcluir []string // IATA
	DuracaoMaxHoras   float64  // 0 = sem limite
	PrecoAlvoCentavos int64    // limiar do alerta "abaixo do alvo"
}

// Aceita decide se uma oferta passa nos filtros configurados.
func (f Filtros) Aceita(o Offer) bool {
	if f.SomenteDireto && o.Paradas > 0 {
		return false
	}
	if f.MaxParadas > 0 && o.Paradas > f.MaxParadas {
		return false
	}
	if f.DuracaoMaxHoras > 0 && o.DuracaoHoras > f.DuracaoMaxHoras {
		return false
	}
	if len(f.CompanhiasIncluir) > 0 && !slices.Contains(f.CompanhiasIncluir, o.Companhia) {
		return false
	}
	if slices.Contains(f.CompanhiasExcluir, o.Companhia) {
		return false
	}
	return true
}

// Chave identifica unicamente uma combinação rota+datas.
type Chave struct {
	Origem  string
	Destino string
	Ida     string
	Volta   string
}

// String serializa a chave no formato usado como índice do histórico/estado.
func (c Chave) String() string {
	return c.Origem + "|" + c.Destino + "|" + c.Ida + "|" + c.Volta
}

// RegistroPreco é uma linha do histórico de preços (NDJSON).
type RegistroPreco struct {
	TS            time.Time
	Origem        string
	Destino       string
	Ida           string
	Volta         string
	PrecoCentavos int64
	Moeda         string
	Companhia     string
	Paradas       int
	Fonte         string
}

// Chave da combinação a que este registro pertence.
func (r RegistroPreco) Chave() Chave {
	return Chave{Origem: r.Origem, Destino: r.Destino, Ida: r.Ida, Volta: r.Volta}
}

// RegistroDe converte a oferta mais barata num registro de histórico.
func RegistroDe(o Offer, ts time.Time) RegistroPreco {
	return RegistroPreco{
		TS:            ts,
		Origem:        o.Origem,
		Destino:       o.Destino,
		Ida:           o.Ida,
		Volta:         o.Volta,
		PrecoCentavos: o.PrecoCentavos,
		Moeda:         o.Moeda,
		Companhia:     o.Companhia,
		Paradas:       o.Paradas,
		Fonte:         o.Fonte,
	}
}

// OfferProvider busca ofertas para uma Busca. Implementado por adapters/skyscanner.
type OfferProvider interface {
	Buscar(ctx context.Context, b Busca) ([]Offer, error)
}

// Repository persiste e lê o histórico de preços. Implementado por infrastructure/store.
type Repository interface {
	Salvar(ctx context.Context, r RegistroPreco) error
	Historico(ctx context.Context, chave Chave) ([]RegistroPreco, error)
}

// Notifier envia uma mensagem de alerta. Implementado por adapters/telegram.
type Notifier interface {
	Avisar(ctx context.Context, msg string) error
}
