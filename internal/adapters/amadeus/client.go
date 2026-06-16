// Package amadeus implementa flight.OfferProvider sobre a Amadeus
// Self-Service API (OAuth2 client_credentials + Flight Offers Search).
package amadeus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DevKayoS/voo-radar/internal/domain/flight"
	"github.com/DevKayoS/voo-radar/internal/utils"
)

const (
	baseTest = "https://test.api.amadeus.com"
	baseProd = "https://api.amadeus.com"
)

// Client é um cliente da Amadeus que faz cache do token de acesso por execução.
type Client struct {
	baseURL      string
	fonte        string
	clientID     string
	clientSecret string
	http         *http.Client

	token    string
	tokenExp time.Time
}

// NewClient cria um cliente para o ambiente "test" ou "prod".
func NewClient(env, clientID, clientSecret string) *Client {
	base := baseTest
	if env == "prod" {
		base = baseProd
	}
	return &Client{
		baseURL:      base,
		fonte:        "amadeus-" + env,
		clientID:     clientID,
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 25 * time.Second},
	}
}

// Buscar consulta as ofertas para uma busca e mapeia para o domínio.
func (c *Client) Buscar(ctx context.Context, b flight.Busca) ([]flight.Offer, error) {
	if err := c.garantirToken(ctx); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("originLocationCode", b.Origem)
	q.Set("destinationLocationCode", b.Destino)
	q.Set("departureDate", b.Ida)
	q.Set("returnDate", b.Volta)
	q.Set("adults", strconv.Itoa(b.Adultos))
	q.Set("currencyCode", b.Moeda)
	q.Set("max", "10")
	if b.Filtros.SomenteDireto {
		q.Set("nonStop", "true")
	}
	if len(b.Filtros.CompanhiasIncluir) > 0 {
		q.Set("includedAirlineCodes", strings.Join(b.Filtros.CompanhiasIncluir, ","))
	} else if len(b.Filtros.CompanhiasExcluir) > 0 {
		// a Amadeus não aceita included + excluded juntos
		q.Set("excludedAirlineCodes", strings.Join(b.Filtros.CompanhiasExcluir, ","))
	}

	endpoint := c.baseURL + "/v2/shopping/flight-offers?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("amadeus: montar request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("amadeus: buscar ofertas: %w", err)
	}
	defer resp.Body.Close()

	var out offersResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("amadeus: decodificar resposta (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("amadeus: status %d: %s", resp.StatusCode, primeiroErro(out.Errors))
	}

	return c.mapearOfertas(b, out.Data), nil
}

// garantirToken obtém (ou reaproveita) um token de acesso OAuth2.
func (c *Client) garantirToken(ctx context.Context) error {
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return nil
	}
	if c.clientID == "" || c.clientSecret == "" {
		return fmt.Errorf("amadeus: credenciais ausentes (AMADEUS_CLIENT_ID/SECRET)")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	endpoint := c.baseURL + "/v1/security/oauth2/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("amadeus: montar request de token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("amadeus: pedir token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("amadeus: token status %d", resp.StatusCode)
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return fmt.Errorf("amadeus: decodificar token: %w", err)
	}

	c.token = tok.AccessToken
	// margem de segurança de 60s antes do vencimento real
	c.tokenExp = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)
	return nil
}

// mapearOfertas converte a resposta crua da Amadeus em []flight.Offer.
func (c *Client) mapearOfertas(b flight.Busca, dados []offerData) []flight.Offer {
	ofertas := make([]flight.Offer, 0, len(dados))
	for _, d := range dados {
		total, err := strconv.ParseFloat(d.Price.Total, 64)
		if err != nil {
			continue
		}
		ofertas = append(ofertas, flight.Offer{
			Origem:        b.Origem,
			Destino:       b.Destino,
			Ida:           b.Ida,
			Volta:         b.Volta,
			PrecoCentavos: utils.ToCentavos(total),
			Moeda:         orDefault(d.Price.Currency, b.Moeda),
			Companhia:     companhiaValidadora(d),
			Paradas:       maxParadas(d.Itineraries),
			DuracaoHoras:  maxDuracao(d.Itineraries),
			Fonte:         c.fonte,
		})
	}
	return ofertas
}

// companhiaValidadora pega a primeira cia validadora, ou o carrier do 1º segmento.
func companhiaValidadora(d offerData) string {
	if len(d.ValidatingAirlineCodes) > 0 {
		return d.ValidatingAirlineCodes[0]
	}
	if len(d.Itineraries) > 0 && len(d.Itineraries[0].Segments) > 0 {
		return d.Itineraries[0].Segments[0].CarrierCode
	}
	return ""
}

// maxParadas devolve o maior número de paradas entre os trechos (ida/volta).
// Paradas = conexões (segmentos-1) + escalas técnicas de cada segmento.
func maxParadas(its []itinerary) int {
	maior := 0
	for _, it := range its {
		paradas := len(it.Segments) - 1
		for _, s := range it.Segments {
			paradas += s.NumberOfStops
		}
		if paradas > maior {
			maior = paradas
		}
	}
	return maior
}

// maxDuracao devolve a maior duração (em horas) entre os trechos.
func maxDuracao(its []itinerary) float64 {
	var maior float64
	for _, it := range its {
		if h := parseDuracaoISO(it.Duration); h > maior {
			maior = h
		}
	}
	return maior
}

// parseDuracaoISO converte uma duração ISO-8601 ("PT9H30M") em horas.
func parseDuracaoISO(s string) float64 {
	s = strings.TrimPrefix(s, "PT")
	var horas, minutos float64
	num := ""
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			num += string(r)
		case r == 'H':
			horas, _ = strconv.ParseFloat(num, 64)
			num = ""
		case r == 'M':
			minutos, _ = strconv.ParseFloat(num, 64)
			num = ""
		default:
			num = ""
		}
	}
	return horas + minutos/60.0
}

func primeiroErro(errs []apiError) string {
	if len(errs) == 0 {
		return "sem detalhe"
	}
	return errs[0].Title + " — " + errs[0].Detail
}

func orDefault(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
