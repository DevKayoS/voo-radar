# voo-radar — Especificação

Bot que monitora preços de passagem aérea e avisa no Telegram quando acha um preço bom.
Roda sozinho via **GitHub Actions (cron)** — sem servidor/VPS pra manter.

> **Status:** spec. Nenhum código escrito ainda. Este documento é a fonte da verdade
> antes de implementar. Tudo aqui é revisável — é pra discutir antes de codar.

**Module:** `github.com/DevKayoS/voo-radar`
**Go:** 1.26
**Arquitetura:** DDD enxuto (mesma pegada do `fintech-kodify`)

---

## 1. Objetivo concreto (caso de uso real)

Monitorar a passagem **São Paulo → Santiago (Chile)** pra uma viagem em outubro/2026
e ser avisado no Telegram quando o preço cair abaixo de um alvo ou bater mínima histórica.

Parâmetros reais da viagem (já viram a config padrão):

| Item | Valor |
|---|---|
| Origem | `GRU` (Guarulhos) |
| Destino | `SCL` (Santiago) |
| Ida | `2026-10-12` **ou** `2026-10-13` |
| Volta | `2026-10-17` |
| Passageiros | 1 adulto |
| Moeda | BRL |
| Tipo | Ida e volta (round-trip) |

> Nota: origem fixada em `GRU`. `CGH` (Congonhas) é praticamente só doméstico e `VCP`
> (Campinas) foi descartado por decisão do dono — só Guarulhos.
>
> Tudo isso é **configurável** (`config/buscas.yaml`) — origens, datas, alvos e **filtros**
> (paradas, companhias, etc.) são só editar o arquivo, sem mexer no código (ver §7).

---

## 2. Fonte de dados: Amadeus Self-Service API

Decisão fechada: **API oficial da Amadeus**, não scraping.
Motivo: o bot precisa rodar confiável por meses até outubro; scraper quebra quando o
site muda e some com o histórico sem avisar. Volume é baixíssimo (cabe no free tier).

### Endpoints usados

1. **OAuth2 token** — `POST /v1/security/oauth2/token`
   - `grant_type=client_credentials`, `client_id`, `client_secret`
   - Retorna `access_token` válido por ~30 min. Cada execução pega um token novo.

2. **Flight Offers Search** — `GET /v2/shopping/flight-offers`
   - Params: `originLocationCode`, `destinationLocationCode`, `departureDate`,
     `returnDate`, `adults`, `currencyCode=BRL`, `max=5`, `nonStop` (opcional)
   - Retorna ofertas com preço total e itinerários.

### Ambientes
- **Teste:** `test.api.amadeus.com` — free, porém preços podem vir de cache (não 100% ao vivo).
- **Produção:** `api.amadeus.com` — também tem cota grátis, preços ao vivo.

Começamos no **teste** pra validar; se os preços vierem defasados, troca o `base_url`
na config pra produção. (Decisão por env var `AMADEUS_ENV=test|prod`.)

### Credenciais (GitHub Secrets)
- `AMADEUS_CLIENT_ID`
- `AMADEUS_CLIENT_SECRET`

> ⚠️ Risco conhecido: se o free tier da Amadeus não for suficiente ou os preços de teste
> forem ruins demais, o plano B é plugar uma segunda fonte (Kiwi/Tequila) ou SerpAPI.
> O `adapters/` é desenhado pra isso ser uma troca isolada (ver §6, interface `OfferProvider`).

---

## 3. Como roda: GitHub Actions (git scraping)

Sem VPS. O fluxo:

```
   cron (a cada 6h)
        │
        ▼
  go run ./cmd/radar
        │
   ┌────┴────────────────────────┐
   ▼                             ▼
 busca preços (Amadeus)    avalia regra de alerta
   │                             │
   ▼                             ▼
 append em data/history.ndjson   se "bom" → Telegram
   │
   ▼
 git commit + push (histórico versionado no próprio repo)
```

Esse é o padrão **"git scraping"**: a cada execução o job acrescenta uma linha no arquivo
de histórico e dá `git push` de volta. Vantagens: zero infra, histórico versionado e
auditável, diffs legíveis. O dashboard (fase futura) lê esse mesmo arquivo.

**Por que NDJSON e não SQLite:** arquivo `.ndjson` (um JSON por linha, append-only) gera
diff de git legível e é trivial de ler tanto em Go quanto em Python/Streamlit depois.
SQLite seria binário no git (diff ruim). Se um dia quisermos dashboard hosted com banco
ao vivo, migra pra **Turso/libSQL** (free tier) — mas não agora.

### Workflow `.github/workflows/coletar.yml` (esboço)
```yaml
name: coletar-precos
on:
  schedule:
    - cron: "0 */6 * * *"   # a cada 6h (horário UTC, GH atrasa às vezes — ok)
  workflow_dispatch: {}       # botão de rodar manual
permissions:
  contents: write             # pra dar push do histórico
jobs:
  coletar:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.26" }
      - run: go run ./cmd/radar
        env:
          AMADEUS_CLIENT_ID: ${{ secrets.AMADEUS_CLIENT_ID }}
          AMADEUS_CLIENT_SECRET: ${{ secrets.AMADEUS_CLIENT_SECRET }}
          AMADEUS_ENV: test
          TELEGRAM_BOT_TOKEN: ${{ secrets.TELEGRAM_BOT_TOKEN }}
          TELEGRAM_CHAT_ID: ${{ secrets.TELEGRAM_CHAT_ID }}
      - name: commit histórico
        run: |
          git config user.name "voo-radar-bot"
          git config user.email "bot@users.noreply.github.com"
          git add data/
          git diff --staged --quiet || git commit -m "dados: coleta $(date -u +%FT%TZ)"
          git push
```

> Observação honesta sobre o cron do GitHub Actions: o agendamento é **best-effort** e
> pode atrasar minutos (às vezes mais em horário de pico). Pra monitorar preço isso é
> totalmente aceitável. Também há o limite de 2000 min/mês de Actions no plano free —
> cada run leva segundos, então estamos folgadíssimos.

---

## 4. Regra de alerta (quando avisar no Telegram)

Pra cada combinação (origem × data_ida × data_volta) pegamos a **oferta mais barata**.
Dispara alerta no Telegram quando QUALQUER uma for verdadeira:

1. **Abaixo do alvo:** `preco <= preco_alvo_centavos` da config.
2. **Mínima histórica:** `preco < menor preço já registrado` pra aquela combinação.
3. **Queda relevante:** `preco <= media_recente * (1 - limiar)` (ex.: 15% abaixo da média
   dos últimos N registros). *(Opcional — pode entrar na fase 2.)*

**Anti-spam (importante):** não mandar a mesma mensagem toda execução.
Guardamos o estado do último alerta por combinação (`data/alert_state.json`):
só avisa de novo se o preço **melhorou** desde o último alerta, ou se voltou a cruzar o
alvo depois de ter subido. Mínima histórica sempre vale a pena avisar.

### Formato da mensagem (Markdown)
```
✈️ *Preço bom achado!*

GRU → SCL  (ida e volta)
📅 12/out → 17/out
💰 *R$ 2.180,00*  (alvo: R$ 2.500,00)
📉 menor preço dos últimos 30 dias

Cia: LATAM · 1 parada
Visto em: 15/06 14:32
```

---

## 5. Modelo de dados

### `data/history.ndjson` (append-only, 1 linha por coleta de combinação)
```json
{"ts":"2026-06-15T14:32:00Z","origem":"GRU","destino":"SCL","ida":"2026-10-12","volta":"2026-10-17","preco_centavos":218000,"moeda":"BRL","companhia":"LATAM","paradas":1,"fonte":"amadeus-test"}
```

### `data/alert_state.json` (estado pra anti-spam)
```json
{
  "GRU|SCL|2026-10-12|2026-10-17": {"ultimo_alerta_centavos":218000,"ts":"2026-06-15T14:32:00Z"}
}
```

Preço sempre em **centavos (`int64`)** — mesma convenção do `fintech-kodify` (`utils.ToReais/ToCentavos`).

---

## 6. Arquitetura e estrutura de diretórios

DDD enxuto, espelhando o `fintech-kodify`, mas sem banco/HTTP server (é um job CLI).

```
voo-radar/
├── cmd/
│   └── radar/
│       └── main.go                 # entry point: lê config, roda coleta, fecha
├── internal/
│   ├── domain/
│   │   └── flight/
│   │       └── entity.go           # Offer, Busca, RegistroPreco + interfaces (Repository, OfferProvider, Notifier)
│   ├── usecases/
│   │   ├── collect/
│   │   │   └── collect_usecase.go  # orquestra: provider.Buscar → repo.Salvar → avalia alerta → notifier
│   │   └── alert/
│   │       └── alert_rule.go       # regras §4 (abaixo do alvo, mínima histórica, queda %)
│   ├── adapters/
│   │   ├── amadeus/
│   │   │   ├── client.go           # OfferProvider: OAuth2 + flight-offers; mapeia p/ domain
│   │   │   └── models.go           # structs do JSON da Amadeus
│   │   └── telegram/
│   │       ├── responder.go        # Notifier: sendMessage (igual fintech-kodify)
│   │       └── models.go           # TelegramSendMessage
│   ├── infrastructure/
│   │   └── store/
│   │       ├── ndjson_repo.go      # Repository: append/ler history.ndjson
│   │       └── alert_state.go      # ler/gravar alert_state.json
│   ├── config/
│   │   └── config.go               # carrega buscas.yaml + env vars
│   └── utils/
│       ├── money.go                # ToReais, ToCentavos, FormatBRL
│       └── date.go                 # parsing/formatação de datas
├── config/
│   └── buscas.yaml                 # config das rotas/datas/alvos (versionado)
├── data/                           # gerado/commitado pela Action
│   ├── history.ndjson
│   └── alert_state.json
├── .github/workflows/coletar.yml
├── .env.example
├── go.mod
├── Makefile
├── README.md
└── SPEC.md
```

### Contratos de domínio (interfaces — o que permite trocar a fonte)
```go
// OfferProvider: implementado por adapters/amadeus (e futuros: kiwi, serpapi)
type OfferProvider interface {
    Buscar(ctx context.Context, b Busca) ([]Offer, error)
}

// Repository: implementado por infrastructure/store
type Repository interface {
    Salvar(ctx context.Context, r RegistroPreco) error
    Historico(ctx context.Context, chave Chave) ([]RegistroPreco, error)
}

// Notifier: implementado por adapters/telegram
type Notifier interface {
    Avisar(ctx context.Context, msg string) error
}
```

**Regra de dependência:** `adapters` e `infrastructure` dependem de `domain`;
`usecases` orquestram via as interfaces; `domain` não importa nada de fora.

---

## 7. Configuração

### `config/buscas.yaml`
```yaml
moeda: BRL
adultos: 1
amadeus_env: test          # test | prod

# Defaults aplicados a toda busca (cada busca pode sobrescrever)
filtros_padrao:
  max_paradas: 2           # 0 = só direto; 1, 2...
  somente_direto: false
  companhias_incluir: []   # ex.: [LA, G3] — vazio = todas (códigos IATA da cia)
  companhias_excluir: []   # ex.: [O6]
  duracao_max_horas: 0     # 0 = sem limite de duração total do trecho

buscas:
  - origens: [GRU]
    destino: SCL
    datas_ida:   ["2026-10-12", "2026-10-13"]
    datas_volta: ["2026-10-17"]
    preco_alvo_reais: 2500
    # filtros: {}           # opcional: sobrescreve filtros_padrao só nesta busca
```

**Filtros configuráveis** (a pedido — nenhum hardcoded no código):

| Filtro | O que faz | Onde aplica |
|---|---|---|
| `max_paradas` | descarta ofertas com mais que N conexões | pós-busca (Amadeus retorna `numberOfStops`) |
| `somente_direto` | atalho pra `max_paradas: 0` → vira param `nonStop=true` na API | na query |
| `companhias_incluir` | só considera essas cias (IATA) | na query (`includedAirlineCodes`) |
| `companhias_excluir` | remove essas cias | na query (`excludedAirlineCodes`) |
| `duracao_max_horas` | descarta itinerários muito longos | pós-busca |
| `preco_alvo_reais` | limiar do alerta "abaixo do alvo" (§4) | regra de alerta |

> Decisão de design: filtros que a Amadeus aceita como parâmetro (cias, nonStop) vão
> direto na query (economiza cota); o resto (`max_paradas>0`, duração) é filtrado em Go
> sobre o resultado. Tudo lido do YAML — trocar filtro nunca exige recompilar lógica.

O carregador expande cada busca na matriz `origens × datas_ida × datas_volta`.
Config atual = 1 × 2 × 1 = **2 consultas** por execução.

### Variáveis de ambiente (`.env.example`)
```
AMADEUS_CLIENT_ID=
AMADEUS_CLIENT_SECRET=
AMADEUS_ENV=test
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

---

## 8. Dependências

Mínimo possível, perto da stdlib (estilo do `fintech-kodify`):
- `net/http`, `encoding/json`, `log/slog` — stdlib (Amadeus + Telegram)
- `gopkg.in/yaml.v3` — ler `buscas.yaml`
- Sem ORM, sem banco, sem framework web.

---

## 9. Plano de implementação (fases)

**Fase 1 — Coletor + Telegram (MVP, é o que destrava tudo)**
1. `config` (carregar yaml + env) e `domain/flight` (entities + interfaces)
2. `utils/money` e `utils/date`
3. `adapters/amadeus` (OAuth2 + flight-offers → `[]Offer`)
4. `infrastructure/store` (append/ler NDJSON + alert_state)
5. `usecases/alert` (regra: abaixo do alvo + mínima histórica)
6. `adapters/telegram` (sendMessage)
7. `usecases/collect` (orquestra tudo) + `cmd/radar/main.go`
8. `.github/workflows/coletar.yml`
9. Testar local com `make run` e credenciais reais, depois ligar no Actions

**Fase 2 — Refino dos alertas**
- Regra de "queda % vs média recente", janela de N dias, formatação melhor da mensagem.

**Fase 3 — Dashboard**
- Lê o `history.ndjson`. Decisão de stack adiada (Go+templ/HTMX, ou Streamlit lendo o
  arquivo, ou migrar histórico pra Turso e servir dashboard hosted). Fora do MVP.

---

## 10. Decisões

**Fechadas:**
- ✅ **Nome:** `voo-radar`.
- ✅ **Origem:** só `GRU` (VCP e CGH descartados).
- ✅ **Visibilidade:** repo **público**. Histórico de preços fica aberto (ok pro dono).
- ✅ **Filtros:** configuráveis via `buscas.yaml`, nada hardcoded (ver §7).
- ✅ **Frequência:** cron 6h/6h (4×/dia) — suficiente, evita ruído.

**Pendentes (só pra hora de ligar a Fase 1):**
- ⏳ **Bot do Telegram:** criar via @BotFather → pegar `TELEGRAM_BOT_TOKEN` e o seu
  `TELEGRAM_CHAT_ID`. Posso te guiar no passo a passo quando o código estiver pronto.
- ⏳ **Credenciais Amadeus:** criar conta dev e pegar `CLIENT_ID`/`CLIENT_SECRET`.
```
