package skyscanner

// ---- searchAirport ----

type airportResponse struct {
	Status bool           `json:"status"`
	Data   []airportEntry `json:"data"`
}

type airportEntry struct {
	SkyID      string     `json:"skyId"`
	EntityID   string     `json:"entityId"`
	Navigation navigation `json:"navigation"`
}

type navigation struct {
	RelevantFlightParams flightParams `json:"relevantFlightParams"`
}

type flightParams struct {
	SkyID    string `json:"skyId"`
	EntityID string `json:"entityId"`
}

// ids devolve skyId/entityId, preferindo os de relevantFlightParams.
func (e airportEntry) ids() (skyID, entityID string) {
	skyID, entityID = e.SkyID, e.EntityID
	if e.Navigation.RelevantFlightParams.SkyID != "" {
		skyID = e.Navigation.RelevantFlightParams.SkyID
	}
	if e.Navigation.RelevantFlightParams.EntityID != "" {
		entityID = e.Navigation.RelevantFlightParams.EntityID
	}
	return skyID, entityID
}

// ---- searchFlights ----

type flightsResponse struct {
	Status bool        `json:"status"`
	Data   flightsData `json:"data"`
}

type flightsData struct {
	Itineraries []itinerary `json:"itineraries"`
}

type itinerary struct {
	Price priceRaw `json:"price"`
	Legs  []leg    `json:"legs"`
}

type priceRaw struct {
	Raw float64 `json:"raw"` // já na moeda pedida (ex.: BRL)
}

type leg struct {
	DurationInMinutes int      `json:"durationInMinutes"`
	StopCount         int      `json:"stopCount"`
	Carriers          carriers `json:"carriers"`
}

type carriers struct {
	Marketing []carrier `json:"marketing"`
}

type carrier struct {
	Name        string `json:"name"`
	AlternateID string `json:"alternateId"` // costuma ser o código IATA (ex.: "LA")
}
