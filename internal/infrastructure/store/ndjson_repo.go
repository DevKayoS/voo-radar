// Package store implementa a persistência em arquivo (padrão "git scraping"):
// histórico append-only em NDJSON e estado de alerta em JSON.
package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DevKayoS/voo-radar/internal/domain/flight"
)

// NDJSONRepo persiste o histórico de preços, uma linha JSON por coleta.
type NDJSONRepo struct {
	caminho string
	mu      sync.Mutex
}

// NewNDJSONRepo cria o repositório apontando para o arquivo de histórico.
func NewNDJSONRepo(caminho string) *NDJSONRepo {
	return &NDJSONRepo{caminho: caminho}
}

// linha é o formato serializado de cada registro no NDJSON.
type linha struct {
	TS            string `json:"ts"`
	Origem        string `json:"origem"`
	Destino       string `json:"destino"`
	Ida           string `json:"ida"`
	Volta         string `json:"volta"`
	PrecoCentavos int64  `json:"preco_centavos"`
	Moeda         string `json:"moeda"`
	Companhia     string `json:"companhia"`
	Paradas       int    `json:"paradas"`
	Fonte         string `json:"fonte"`
}

// Salvar acrescenta um registro ao final do arquivo de histórico.
func (r *NDJSONRepo) Salvar(_ context.Context, rec flight.RegistroPreco) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.caminho), 0o755); err != nil {
		return fmt.Errorf("store: criar diretório: %w", err)
	}

	f, err := os.OpenFile(r.caminho, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("store: abrir histórico: %w", err)
	}
	defer f.Close()

	dados, err := json.Marshal(deRegistro(rec))
	if err != nil {
		return fmt.Errorf("store: serializar registro: %w", err)
	}
	if _, err := f.Write(append(dados, '\n')); err != nil {
		return fmt.Errorf("store: escrever registro: %w", err)
	}
	return nil
}

// Historico devolve todos os registros de uma combinação rota+datas.
func (r *NDJSONRepo) Historico(_ context.Context, chave flight.Chave) ([]flight.RegistroPreco, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := os.Open(r.caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // ainda não há histórico
		}
		return nil, fmt.Errorf("store: abrir histórico: %w", err)
	}
	defer f.Close()

	var registros []flight.RegistroPreco
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		texto := sc.Bytes()
		if len(texto) == 0 {
			continue
		}
		var l linha
		if err := json.Unmarshal(texto, &l); err != nil {
			continue // pula linhas corrompidas em vez de abortar
		}
		rec := paraRegistro(l)
		if rec.Chave() == chave {
			registros = append(registros, rec)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("store: ler histórico: %w", err)
	}
	return registros, nil
}

func deRegistro(r flight.RegistroPreco) linha {
	return linha{
		TS:            r.TS.UTC().Format(time.RFC3339),
		Origem:        r.Origem,
		Destino:       r.Destino,
		Ida:           r.Ida,
		Volta:         r.Volta,
		PrecoCentavos: r.PrecoCentavos,
		Moeda:         r.Moeda,
		Companhia:     r.Companhia,
		Paradas:       r.Paradas,
		Fonte:         r.Fonte,
	}
}

func paraRegistro(l linha) flight.RegistroPreco {
	ts, _ := time.Parse(time.RFC3339, l.TS)
	return flight.RegistroPreco{
		TS:            ts,
		Origem:        l.Origem,
		Destino:       l.Destino,
		Ida:           l.Ida,
		Volta:         l.Volta,
		PrecoCentavos: l.PrecoCentavos,
		Moeda:         l.Moeda,
		Companhia:     l.Companhia,
		Paradas:       l.Paradas,
		Fonte:         l.Fonte,
	}
}
