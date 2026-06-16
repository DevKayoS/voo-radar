// Package skyscanner implementa flight.OfferProvider sobre a API "Sky Scrapper"
// do RapidAPI (dados do Skyscanner). Fluxo: searchAirport (resolve IDs, com
// cache em disco) + searchFlights (busca ida-e-volta via returnDate).
package skyscanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DevKayoS/voo-radar/internal/domain/flight"
	"github.com/DevKayoS/voo-radar/internal/utils"
)

const host = "sky-scrapper.p.rapidapi.com"

// Client consulta o Sky Scrapper usando uma chave do RapidAPI.
type Client struct {
	apiKey string
	market string // locale, ex.: "en-US"
	pais   string // countryCode, ex.: "BR"
	http   *http.Client
	cache  *cacheAeroportos
}

// NewClient cria o cliente; cacheCaminho é onde os IDs de aeroporto são guardados.
func NewClient(apiKey, cacheCaminho string) *Client {
	return &Client{
		apiKey: apiKey,
		market: "en-US",
		pais:   "BR",
		http:   &http.Client{Timeout: 30 * time.Second},
		cache:  carregarCacheAeroportos(cacheCaminho),
	}
}

// Buscar resolve os aeroportos e consulta as ofertas de ida-e-volta.
func (c *Client) Buscar(ctx context.Context, b flight.Busca) ([]flight.Offer, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("skyscanner: RAPIDAPI_KEY ausente")
	}

	origem, err := c.resolverAeroporto(ctx, b.Origem)
	if err != nil {
		return nil, err
	}
	destino, err := c.resolverAeroporto(ctx, b.Destino)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("originSkyId", origem.SkyID)
	q.Set("destinationSkyId", destino.SkyID)
	q.Set("originEntityId", origem.EntityID)
	q.Set("destinationEntityId", destino.EntityID)
	q.Set("date", b.Ida)
	q.Set("returnDate", b.Volta)
	q.Set("cabinClass", "economy")
	q.Set("adults", strconv.Itoa(b.Adultos))
	q.Set("sortBy", "price_low")
	q.Set("currency", b.Moeda)
	q.Set("market", c.market)
	q.Set("countryCode", c.pais)

	var resp flightsResponse
	if err := c.get(ctx, "/api/v1/flights/searchFlights", q, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return nil, fmt.Errorf("skyscanner: busca sem sucesso para %s→%s: %s", b.Origem, b.Destino, resp.Message)
	}

	return mapearOfertas(b, resp.Data.Itineraries), nil
}

// SalvarCache persiste o cache de aeroportos (chamado pelo cmd ao final).
func (c *Client) SalvarCache() error {
	return c.cache.Salvar()
}

// resolverAeroporto devolve os IDs do aeroporto, do cache ou via searchAirport.
func (c *Client) resolverAeroporto(ctx context.Context, iata string) (aeroporto, error) {
	if a, ok := c.cache.get(iata); ok {
		return a, nil
	}

	q := url.Values{}
	q.Set("query", iata)
	var resp airportResponse
	if err := c.get(ctx, "/api/v1/flights/searchAirport", q, &resp); err != nil {
		return aeroporto{}, err
	}

	a, err := escolherAeroporto(iata, resp.Data)
	if err != nil {
		return aeroporto{}, err
	}
	c.cache.set(iata, a)
	return a, nil
}

// get faz a chamada HTTP autenticada e decodifica a resposta JSON.
func (c *Client) get(ctx context.Context, caminho string, q url.Values, out any) error {
	endpoint := "https://" + host + caminho + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("skyscanner: montar request: %w", err)
	}
	req.Header.Set("X-RapidAPI-Key", c.apiKey)
	req.Header.Set("X-RapidAPI-Host", host)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("skyscanner: %s: %w", caminho, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return fmt.Errorf("skyscanner: %s status %d: %s", caminho, resp.StatusCode, strings.TrimSpace(string(corpo)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("skyscanner: decodificar %s: %w", caminho, err)
	}
	return nil
}

// escolherAeroporto prefere a entrada cujo skyId bate com o IATA buscado.
func escolherAeroporto(iata string, entradas []airportEntry) (aeroporto, error) {
	if len(entradas) == 0 {
		return aeroporto{}, fmt.Errorf("skyscanner: aeroporto %q não encontrado", iata)
	}
	for _, e := range entradas {
		if sky, ent := e.ids(); strings.EqualFold(sky, iata) {
			return aeroporto{SkyID: sky, EntityID: ent}, nil
		}
	}
	sky, ent := entradas[0].ids()
	return aeroporto{SkyID: sky, EntityID: ent}, nil
}

// mapearOfertas converte os itinerários do Skyscanner em []flight.Offer.
func mapearOfertas(b flight.Busca, its []itinerary) []flight.Offer {
	ofertas := make([]flight.Offer, 0, len(its))
	for _, it := range its {
		if len(it.Legs) == 0 || it.Price.Raw <= 0 {
			continue
		}
		ofertas = append(ofertas, flight.Offer{
			Origem:        b.Origem,
			Destino:       b.Destino,
			Ida:           b.Ida,
			Volta:         b.Volta,
			PrecoCentavos: utils.ToCentavos(it.Price.Raw),
			Moeda:         b.Moeda,
			Companhia:     companhiaDe(it.Legs[0]),
			Paradas:       maxStops(it.Legs),
			DuracaoHoras:  maxDuracaoHoras(it.Legs),
			Fonte:         "skyscanner",
		})
	}
	return ofertas
}

func companhiaDe(l leg) string {
	if len(l.Carriers.Marketing) == 0 {
		return ""
	}
	m := l.Carriers.Marketing[0]
	if m.AlternateID != "" {
		return m.AlternateID
	}
	return m.Name
}

func maxStops(legs []leg) int {
	maior := 0
	for _, l := range legs {
		if l.StopCount > maior {
			maior = l.StopCount
		}
	}
	return maior
}

func maxDuracaoHoras(legs []leg) float64 {
	maior := 0
	for _, l := range legs {
		if l.DurationInMinutes > maior {
			maior = l.DurationInMinutes
		}
	}
	return float64(maior) / 60.0
}
