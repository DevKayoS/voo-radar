// Command radar executa uma rodada de coleta de preços de passagem:
// lê a config, consulta a Amadeus, registra o histórico e alerta no Telegram.
// Pensado para rodar via cron do GitHub Actions.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/DevKayoS/voo-radar/internal/adapters/amadeus"
	"github.com/DevKayoS/voo-radar/internal/adapters/telegram"
	"github.com/DevKayoS/voo-radar/internal/config"
	"github.com/DevKayoS/voo-radar/internal/infrastructure/store"
	"github.com/DevKayoS/voo-radar/internal/usecases/collect"
)

const (
	caminhoConfig    = "config/buscas.yaml"
	caminhoHistorico = "data/history.ndjson"
	caminhoEstado    = "data/alert_state.json"
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
	slog.Info("radar: config carregada",
		"buscas", len(cfg.Buscas), "amadeus_env", cfg.Amadeus.Env)

	provider := amadeus.NewClient(cfg.Amadeus.Env, cfg.Amadeus.ClientID, cfg.Amadeus.ClientSecret)
	repo := store.NewNDJSONRepo(caminhoHistorico)
	notifier := telegram.NewNotifier(cfg.Telegram.BotToken, cfg.Telegram.ChatID)

	estado, err := store.CarregarAlertState(caminhoEstado)
	if err != nil {
		return err
	}

	uc := collect.NewUseCase(provider, repo, notifier, estado)
	return uc.Run(ctx, cfg.Buscas)
}
