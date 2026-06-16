# voo-radar ✈️

Bot que monitora preços de passagem aérea e avisa no Telegram quando acha um preço bom.
Roda sozinho via **GitHub Actions (cron)** — sem servidor/VPS pra manter.

Caso de uso inicial: **GRU → SCL** (São Paulo → Santiago) em outubro/2026. Tudo é
configurável em [`config/buscas.yaml`](config/buscas.yaml).

> Especificação completa do projeto em [`SPEC.md`](SPEC.md).

## Como funciona

```
cron (6h/6h) → go run ./cmd/radar
  → busca preços (Amadeus API)
  → filtra (paradas, cias, duração — tudo via config)
  → registra em data/history.ndjson  (histórico versionado no git)
  → se preço bom: alerta no Telegram
  → git commit + push dos dados
```

## Arquitetura (DDD enxuto)

```
cmd/radar            entrypoint (rodado pelo Actions)
internal/
  domain/flight      entidades + interfaces (OfferProvider, Repository, Notifier)
  usecases/collect   orquestra o ciclo de coleta + formata a mensagem
  usecases/alert     regra pura de "quando avisar" (testada)
  adapters/amadeus   OfferProvider sobre a Amadeus API (OAuth2 + flight-offers)
  adapters/telegram  Notifier (disparo via Bot API)
  infrastructure/store  histórico NDJSON + estado de alerta (anti-spam)
  config             carrega buscas.yaml + variáveis de ambiente
  utils              dinheiro (centavos) e datas
```

## Rodar localmente

1. Copie o `.env`:
   ```sh
   cp .env.example .env
   # preencha AMADEUS_* e TELEGRAM_* (veja "Credenciais" abaixo)
   ```
2. Rode:
   ```sh
   make run     # carrega o .env e executa uma coleta
   make test    # testes
   make build   # binário em bin/radar
   ```

Sem credenciais, o bot ainda roda: carrega a config e loga, só não consulta a API
nem envia Telegram (útil pra validar a estrutura).

## Credenciais

**Amadeus** — crie conta em https://developers.amadeus.com, gere uma app e pegue
`API Key` (→ `AMADEUS_CLIENT_ID`) e `API Secret` (→ `AMADEUS_CLIENT_SECRET`).
Comece no ambiente `test` (grátis); se os preços vierem defasados, troque `amadeus_env: prod`.

**Telegram** — fale com o [@BotFather](https://t.me/BotFather), `/newbot`, pegue o token
(`TELEGRAM_BOT_TOKEN`). Para o `TELEGRAM_CHAT_ID`, mande uma mensagem ao seu bot e acesse
`https://api.telegram.org/bot<TOKEN>/getUpdates` — o `chat.id` aparece no JSON.

## Ligar no GitHub Actions

1. Suba o repo no GitHub (público).
2. Em **Settings → Secrets and variables → Actions**, adicione:
   `AMADEUS_CLIENT_ID`, `AMADEUS_CLIENT_SECRET`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`.
3. O workflow [`coletar.yml`](.github/workflows/coletar.yml) roda a cada 6h (ou manualmente
   em **Actions → coletar-precos → Run workflow**) e commita o histórico em `data/`.

## Configuração

Veja [`config/buscas.yaml`](config/buscas.yaml). Filtros configuráveis (nada hardcoded):
`max_paradas`, `somente_direto`, `companhias_incluir/excluir`, `duracao_max_horas`,
`preco_alvo_reais`. Cada busca expande na matriz `origens × datas_ida × datas_volta`.
