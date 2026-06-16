package amadeus

// tokenResponse é a resposta do endpoint OAuth2 de client_credentials.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// offersResponse é a resposta de GET /v2/shopping/flight-offers.
type offersResponse struct {
	Data   []offerData `json:"data"`
	Errors []apiError  `json:"errors"`
}

type apiError struct {
	Status int    `json:"status"`
	Code   int    `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type offerData struct {
	Price                  price       `json:"price"`
	Itineraries            []itinerary `json:"itineraries"`
	ValidatingAirlineCodes []string    `json:"validatingAirlineCodes"`
}

type price struct {
	Total    string `json:"total"`    // ex.: "2180.00"
	Currency string `json:"currency"` // ex.: "BRL"
}

type itinerary struct {
	Duration string    `json:"duration"` // ISO-8601, ex.: "PT9H30M"
	Segments []segment `json:"segments"`
}

type segment struct {
	NumberOfStops int      `json:"numberOfStops"` // escalas técnicas (sem troca de avião)
	CarrierCode   string   `json:"carrierCode"`
	Departure     endpoint `json:"departure"`
	Arrival       endpoint `json:"arrival"`
}

type endpoint struct {
	IataCode string `json:"iataCode"`
	At       string `json:"at"`
}
