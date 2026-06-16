package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AlertState guarda, por combinação, o último preço já alertado. Serve de
// anti-spam: só avisamos de novo quando o preço melhora (ver usecases/alert).
type AlertState struct {
	caminho string
	dados   map[string]estadoAlerta
}

type estadoAlerta struct {
	UltimoAlertaCentavos int64  `json:"ultimo_alerta_centavos"`
	TS                   string `json:"ts"`
}

// CarregarAlertState lê o estado do disco; arquivo inexistente vira estado vazio.
func CarregarAlertState(caminho string) (*AlertState, error) {
	s := &AlertState{caminho: caminho, dados: map[string]estadoAlerta{}}

	dados, err := os.ReadFile(caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("store: ler alert_state: %w", err)
	}
	if len(dados) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(dados, &s.dados); err != nil {
		return nil, fmt.Errorf("store: parse alert_state: %w", err)
	}
	return s, nil
}

// Get devolve o último preço alertado para a chave e se ele existe.
func (s *AlertState) Get(chave string) (int64, bool) {
	e, ok := s.dados[chave]
	return e.UltimoAlertaCentavos, ok
}

// Set registra o preço alertado para a chave no instante dado.
func (s *AlertState) Set(chave string, centavos int64, ts time.Time) {
	s.dados[chave] = estadoAlerta{
		UltimoAlertaCentavos: centavos,
		TS:                   ts.UTC().Format(time.RFC3339),
	}
}

// Salvar persiste o estado no disco (JSON indentado, fácil de versionar).
func (s *AlertState) Salvar() error {
	if err := os.MkdirAll(filepath.Dir(s.caminho), 0o755); err != nil {
		return fmt.Errorf("store: criar diretório: %w", err)
	}
	dados, err := json.MarshalIndent(s.dados, "", "  ")
	if err != nil {
		return fmt.Errorf("store: serializar alert_state: %w", err)
	}
	if err := os.WriteFile(s.caminho, append(dados, '\n'), 0o644); err != nil {
		return fmt.Errorf("store: escrever alert_state: %w", err)
	}
	return nil
}
