package skyscanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// aeroporto guarda os identificadores que o Skyscanner exige na busca.
type aeroporto struct {
	SkyID    string `json:"sky_id"`
	EntityID string `json:"entity_id"`
}

// cacheAeroportos mapeia IATA -> identificadores, persistido em disco para
// não gastar cota com searchAirport a cada execução (resolve uma vez na vida).
type cacheAeroportos struct {
	caminho string
	mu      sync.Mutex
	dados   map[string]aeroporto
	sujo    bool
}

func carregarCacheAeroportos(caminho string) *cacheAeroportos {
	c := &cacheAeroportos{caminho: caminho, dados: map[string]aeroporto{}}
	if dados, err := os.ReadFile(caminho); err == nil && len(dados) > 0 {
		_ = json.Unmarshal(dados, &c.dados) // cache corrompido vira cache vazio
	}
	return c
}

func (c *cacheAeroportos) get(iata string) (aeroporto, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, ok := c.dados[iata]
	return a, ok
}

func (c *cacheAeroportos) set(iata string, a aeroporto) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dados[iata] = a
	c.sujo = true
}

// Salvar grava o cache apenas se houve mudança.
func (c *cacheAeroportos) Salvar() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.sujo {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.caminho), 0o755); err != nil {
		return fmt.Errorf("skyscanner: criar diretório do cache: %w", err)
	}
	dados, err := json.MarshalIndent(c.dados, "", "  ")
	if err != nil {
		return fmt.Errorf("skyscanner: serializar cache: %w", err)
	}
	if err := os.WriteFile(c.caminho, append(dados, '\n'), 0o644); err != nil {
		return fmt.Errorf("skyscanner: escrever cache: %w", err)
	}
	c.sujo = false
	return nil
}
