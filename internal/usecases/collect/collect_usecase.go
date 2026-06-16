// Package collect orquestra o ciclo de coleta: buscar ofertas, filtrar,
// registrar no histórico e disparar alerta quando a regra manda.
package collect

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/DevKayoS/voo-radar/internal/domain/flight"
	"github.com/DevKayoS/voo-radar/internal/usecases/alert"
	"github.com/DevKayoS/voo-radar/internal/utils"
)

// EstadoAlerta é o contrato mínimo de anti-spam consumido aqui
// (implementado por infrastructure/store.AlertState).
type EstadoAlerta interface {
	Get(chave string) (int64, bool)
	Set(chave string, centavos int64, ts time.Time)
	Salvar() error
}

// UseCase reúne as dependências do ciclo de coleta.
type UseCase struct {
	provider flight.OfferProvider
	repo     flight.Repository
	notifier flight.Notifier
	estado   EstadoAlerta
	agora    func() time.Time
}

// NewUseCase injeta as dependências (todas via interfaces de domínio).
func NewUseCase(p flight.OfferProvider, r flight.Repository, n flight.Notifier, e EstadoAlerta) *UseCase {
	return &UseCase{
		provider: p,
		repo:     r,
		notifier: n,
		estado:   e,
		agora:    time.Now,
	}
}

// Run executa a coleta para todas as buscas. Falhas isoladas não abortam o
// lote: são logadas e o ciclo continua nas demais buscas.
func (uc *UseCase) Run(ctx context.Context, buscas []flight.Busca) error {
	for _, b := range buscas {
		if err := uc.processar(ctx, b); err != nil {
			slog.Error("coleta: busca falhou",
				"origem", b.Origem, "destino", b.Destino,
				"ida", b.Ida, "volta", b.Volta, "error", err)
		}
	}
	if err := uc.estado.Salvar(); err != nil {
		return fmt.Errorf("coleta: salvar estado de alerta: %w", err)
	}
	return nil
}

func (uc *UseCase) processar(ctx context.Context, b flight.Busca) error {
	ofertas, err := uc.provider.Buscar(ctx, b)
	if err != nil {
		return err
	}

	melhor, ok := maisBarataAceita(b.Filtros, ofertas)
	if !ok {
		slog.Info("coleta: nenhuma oferta após filtros",
			"origem", b.Origem, "destino", b.Destino, "ida", b.Ida, "volta", b.Volta)
		return nil
	}

	chave := b.Chave()
	historico, err := uc.repo.Historico(ctx, chave)
	if err != nil {
		return err
	}
	ultimo, temUltimo := uc.estado.Get(chave.String())

	// Avalia ANTES de salvar (histórico não inclui a oferta atual).
	res := alert.Avaliar(melhor, b.Filtros.PrecoAlvoCentavos, historico, ultimo, temUltimo)

	agora := uc.agora()
	if err := uc.repo.Salvar(ctx, flight.RegistroDe(melhor, agora)); err != nil {
		return err
	}

	slog.Info("coleta: oferta registrada",
		"origem", b.Origem, "destino", b.Destino, "ida", b.Ida, "volta", b.Volta,
		"preco", utils.FormatBRL(melhor.PrecoCentavos), "alerta", res.DeveAvisar)

	if res.DeveAvisar {
		msg := formatarMensagem(melhor, b, res, agora)
		if err := uc.notifier.Avisar(ctx, msg); err != nil {
			return fmt.Errorf("notificar: %w", err)
		}
		uc.estado.Set(chave.String(), melhor.PrecoCentavos, agora)
	}
	return nil
}

// maisBarataAceita devolve a menor oferta que passa nos filtros.
func maisBarataAceita(f flight.Filtros, ofertas []flight.Offer) (flight.Offer, bool) {
	var melhor flight.Offer
	achou := false
	for _, o := range ofertas {
		if !f.Aceita(o) {
			continue
		}
		if !achou || o.PrecoCentavos < melhor.PrecoCentavos {
			melhor = o
			achou = true
		}
	}
	return melhor, achou
}
