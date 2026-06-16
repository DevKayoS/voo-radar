// Package config carrega a configuração de buscas (config/buscas.yaml)
// e as credenciais (variáveis de ambiente), expandindo tudo na lista de
// buscas concretas que o coletor vai consultar.
package config

import (
	"fmt"
	"os"

	"github.com/DevKayoS/voo-radar/internal/domain/flight"
	"github.com/DevKayoS/voo-radar/internal/utils"
	"gopkg.in/yaml.v3"
)

// Config é a configuração já resolvida e pronta para uso.
type Config struct {
	Buscas   []flight.Busca // matriz expandida (origens × datas_ida × datas_volta)
	Amadeus  AmadeusConfig
	Telegram TelegramConfig
}

// AmadeusConfig agrupa credenciais e ambiente da API.
type AmadeusConfig struct {
	ClientID     string
	ClientSecret string
	Env          string // "test" | "prod"
}

// TelegramConfig agrupa o token do bot e o chat de destino.
type TelegramConfig struct {
	BotToken string
	ChatID   string
}

// ---- estruturas espelho do YAML ----

type yamlRoot struct {
	Moeda         string      `yaml:"moeda"`
	Adultos       int         `yaml:"adultos"`
	AmadeusEnv    string      `yaml:"amadeus_env"`
	FiltrosPadrao yamlFiltros `yaml:"filtros_padrao"`
	Buscas        []yamlBusca `yaml:"buscas"`
}

type yamlFiltros struct {
	MaxParadas        int      `yaml:"max_paradas"`
	SomenteDireto     bool     `yaml:"somente_direto"`
	CompanhiasIncluir []string `yaml:"companhias_incluir"`
	CompanhiasExcluir []string `yaml:"companhias_excluir"`
	DuracaoMaxHoras   float64  `yaml:"duracao_max_horas"`
}

type yamlBusca struct {
	Origens        []string     `yaml:"origens"`
	Destino        string       `yaml:"destino"`
	DatasIda       []string     `yaml:"datas_ida"`
	DatasVolta     []string     `yaml:"datas_volta"`
	PrecoAlvoReais float64      `yaml:"preco_alvo_reais"`
	Filtros        *yamlFiltros `yaml:"filtros"`
}

// Carregar lê o arquivo YAML e as variáveis de ambiente e devolve a Config.
func Carregar(caminho string) (*Config, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("config: ler %q: %w", caminho, err)
	}

	var raiz yamlRoot
	if err := yaml.Unmarshal(dados, &raiz); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	moeda := orDefault(raiz.Moeda, "BRL")
	adultos := raiz.Adultos
	if adultos <= 0 {
		adultos = 1
	}

	buscas, err := expandirBuscas(raiz, moeda, adultos)
	if err != nil {
		return nil, err
	}

	// Ambiente da Amadeus: env var sobrescreve o YAML; padrão "test".
	amadeusEnv := orDefault(os.Getenv("AMADEUS_ENV"), orDefault(raiz.AmadeusEnv, "test"))

	return &Config{
		Buscas: buscas,
		Amadeus: AmadeusConfig{
			ClientID:     os.Getenv("AMADEUS_CLIENT_ID"),
			ClientSecret: os.Getenv("AMADEUS_CLIENT_SECRET"),
			Env:          amadeusEnv,
		},
		Telegram: TelegramConfig{
			BotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
			ChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		},
	}, nil
}

// expandirBuscas transforma cada bloco do YAML na matriz origens × ida × volta.
func expandirBuscas(raiz yamlRoot, moeda string, adultos int) ([]flight.Busca, error) {
	var buscas []flight.Busca
	for i, yb := range raiz.Buscas {
		if yb.Destino == "" {
			return nil, fmt.Errorf("config: busca[%d] sem destino", i)
		}
		if len(yb.Origens) == 0 || len(yb.DatasIda) == 0 || len(yb.DatasVolta) == 0 {
			return nil, fmt.Errorf("config: busca[%d] precisa de origens, datas_ida e datas_volta", i)
		}

		filtros := resolverFiltros(raiz.FiltrosPadrao, yb.Filtros, yb.PrecoAlvoReais)

		for _, origem := range yb.Origens {
			for _, ida := range yb.DatasIda {
				for _, volta := range yb.DatasVolta {
					buscas = append(buscas, flight.Busca{
						Origem:  origem,
						Destino: yb.Destino,
						Ida:     ida,
						Volta:   volta,
						Adultos: adultos,
						Moeda:   moeda,
						Filtros: filtros,
					})
				}
			}
		}
	}
	if len(buscas) == 0 {
		return nil, fmt.Errorf("config: nenhuma busca definida")
	}
	return buscas, nil
}

// resolverFiltros aplica os filtros padrão, sobrescritos pelos da busca quando presentes.
func resolverFiltros(padrao yamlFiltros, especifico *yamlFiltros, precoAlvoReais float64) flight.Filtros {
	f := padrao
	if especifico != nil {
		f = *especifico
	}
	return flight.Filtros{
		MaxParadas:        f.MaxParadas,
		SomenteDireto:     f.SomenteDireto,
		CompanhiasIncluir: f.CompanhiasIncluir,
		CompanhiasExcluir: f.CompanhiasExcluir,
		DuracaoMaxHoras:   f.DuracaoMaxHoras,
		PrecoAlvoCentavos: utils.ToCentavos(precoAlvoReais),
	}
}

func orDefault(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
