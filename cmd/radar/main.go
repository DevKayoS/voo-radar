// Command radar executa uma rodada de coleta de preços de passagem:
// lê a config, consulta o Sky Scrapper, registra o histórico e alerta no Telegram.
// Pensado para rodar via cron do GitHub Actions.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/DevKayoS/voo-radar/internal/adapters/skyscanner"
	"github.com/DevKayoS/voo-radar/internal/adapters/telegram"
	"github.com/DevKayoS/voo-radar/internal/config"
	"github.com/DevKayoS/voo-radar/internal/infrastructure/store"
	"github.com/DevKayoS/voo-radar/internal/usecases/collect"
)

const (
	caminhoConfig     = "config/buscas.yaml"
	caminhoHistorico  = "data/history.ndjson"
	caminhoEstado     = "data/alert_state.json"
	caminhoAeroportos = "data/airports.json"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("radar: execução falhou", "error", err)
		os.Exit(1)
	}
	slog.Info("radar: coleta concluída")
}

func run() error {
	ctx := context.Background()

	cfg, err := config.Carregar(caminhoConfig)
	if err != nil {
		return err
	}
	slog.Info("radar: config carregada", "buscas", len(cfg.Buscas))

	provider := skyscanner.NewClient(cfg.RapidAPIKey, caminhoAeroportos)
	repo := store.NewNDJSONRepo(caminhoHistorico)
	notifier := telegram.NewNotifier(cfg.Telegram.BotToken, cfg.Telegram.ChatID)

	estado, err := store.CarregarAlertState(caminhoEstado)
	if err != nil {
		return err
	}

	uc := collect.NewUseCase(provider, repo, notifier, estado)
	if err := uc.Run(ctx, cfg.Buscas); err != nil {
		return err
	}

	// Persiste o cache de aeroportos (economiza cota nas próximas execuções).
	if err := provider.SalvarCache(); err != nil {
		slog.Warn("radar: falha ao salvar cache de aeroportos", "error", err)
	}
	return nil
}
