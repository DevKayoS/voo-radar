// Package telegram implementa flight.Notifier disparando mensagens via
// a Bot API do Telegram (apenas envio, sem receber updates).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Notifier envia mensagens de texto para um chat fixo do Telegram.
type Notifier struct {
	token  string
	chatID string
	http   *http.Client
}

// NewNotifier cria o notificador com o token do bot e o chat de destino.
func NewNotifier(token, chatID string) *Notifier {
	return &Notifier{
		token:  token,
		chatID: chatID,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Avisar envia uma mensagem em Markdown. Se as credenciais não estiverem
// configuradas, apenas loga (permite rodar localmente sem bot).
func (n *Notifier) Avisar(ctx context.Context, text string) error {
	if n.token == "" || n.chatID == "" {
		slog.Warn("telegram: credenciais ausentes, alerta não enviado", "preview", text)
		return nil
	}

	payload := sendMessagePayload{
		ChatID:    n.chatID,
		Text:      text,
		ParseMode: "Markdown",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: serializar mensagem: %w", err)
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: montar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: enviar mensagem: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram: status %d: %s", resp.StatusCode, string(corpo))
	}
	return nil
}
