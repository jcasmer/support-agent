# support-agent

A multi-agent AI support system built in Go using Hexagonal Architecture. Demonstrates real-world agentic AI patterns: orchestration, conversational state management, agent selection logic, controlled degradation, and LLM cost control.

Built as a practical example of the responsibilities found in modern AI engineer roles.

---

## Architecture

This project follows **Hexagonal Architecture** (Ports & Adapters), the most common architecture pattern in production Go services.

```
cmd/
└── server/
    └── main.go               ← composition root — wires all dependencies

internal/
├── core/                     ← domain (pure, no external imports)
│   ├── domain.go             ← business types: State, Result, Ticket, Message
│   ├── agent.go              ← ports: Agent, LLMPort, AgentResult
│   ├── classifier.go         ← ClassifierAgent implementation
│   ├── resolver.go           ← ResolverAgent implementation
│   └── escalator.go          ← EscalatorAgent implementation
├── app/
│   └── orchestrator.go       ← use case: coordinates agent pipeline
└── adapters/
    ├── in/
    │   └── http/
    │       └── handler.go    ← driving adapter: HTTP → app
    └── out/
        ├── anthropic/
        │   └── client.go     ← driven adapter: LLMPort → Anthropic API
        └── gemini/
            └── client.go     ← driven adapter: LLMPort → Gemini API
```

### Dependency Rule

Dependencies only point inward. `core` imports nothing internal. `app` imports only `core`. Adapters import `core` and `app`, never each other.

```
adapters/in  →  app  →  core
adapters/out →  core
```

---

## Agent Pipeline

```
HTTP Request
     │
     ▼
┌─────────────┐
│   Handler   │  ← validates input, calls orchestrator
└──────┬──────┘
       │
       ▼
┌─────────────┐
│Orchestrator │  ← manages flow and state transitions
└──────┬──────┘
       │
  ┌────▼─────┐
  │Classifier│  ← classifies query: billing | technical | general
  └────┬─────┘
       │
  ┌────▼─────┐
  │ Resolver │  ← attempts to resolve the issue
  └────┬─────┘
       │
  resolved? ──yes──→ return response
       │
      no
       │
  ┌────▼─────┐
  │Escalator │  ← generates structured support ticket
  └────┬─────┘
       │
       ▼
  return ticket
```

---

## Key Design Decisions

### State passed by value
`State` is passed by value to every agent — no agent mutates the original. Each agent returns a new state. The orchestrator decides whether to accept it. This makes the flow fully traceable and safe from partial mutations on failure.

### LLMPort — provider-agnostic interface
```go
type LLMPort interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```
Switching from Anthropic to Gemini means adding one file in `adapters/out`. The orchestrator and agents never change.

### Token budget per agent
Each agent declares its own `MaxTokens`. The classifier only needs a single word — it gets 10 tokens. The resolver gets 500. The escalator gets 400. Cost is controlled at the agent level, not globally.

### Explicit escalation signal
The resolver signals inability to resolve by returning `ESCALATE` in its response. The orchestrator reads `state.Resolved` to decide whether to activate the escalator. Agents communicate through state, never by calling each other directly.

### Graceful shutdown
The server waits up to 30 seconds for in-flight requests to complete before shutting down. This prevents cutting LLM calls mid-execution.

---

## API

### POST /v1/support

**Request**
```json
{
  "session_id": "test-001",
  "query": "I cannot login to my account, I keep getting a 401 error"
}
```
`session_id` is optional — the server generates one if not provided.

**Response — resolved**
```json
{
  "session_id": "test-001",
  "response": "Here are the steps to resolve your issue...",
  "category": "technical",
  "resolved": true,
  "tokens_used": 490,
  "duration_ms": 3544
}
```

**Response — escalated**
```json
{
  "session_id": "test-001",
  "response": "Your issue has been escalated to our engineering team...",
  "category": "technical",
  "resolved": false,
  "ticket_id": "TKT-20240523-001",
  "tokens_used": 965,
  "duration_ms": 6575
}
```

**Error envelope**
```json
{
  "code": "missing_field",
  "message": "query is required",
  "request_id": "test-001"
}
```

### GET /health
Returns `200 OK`. Used by load balancers and container orchestrators.

---

## Configuration

| Variable | Required | Default | Description |
|---|---|---|---|
| `ANTHROPIC_API_KEY` | yes | — | Anthropic API key |
| `PORT` | no | `8080` | HTTP server port |

Create a `.env` file in the project root:
```env
ANTHROPIC_API_KEY=sk-ant-xxxxxx
PORT=8080
```

---

## Running the project

```bash
# Install dependencies
go mod download

# Run
go run ./cmd/server

# Build
go build -o support-agent ./cmd/server
```

---

## Switching LLM providers

The system is provider-agnostic by design. To switch to Gemini:

1. Add `GEMINI_API_KEY` to `.env`
2. In `cmd/server/main.go`, replace:
```go
// before
llmClient := anthropic.New(apiKey)

// after
llmClient := gemini.New(apiKey)
```

No other files change.

---

## Agentic patterns demonstrated

| Pattern | Where |
|---|---|
| Agent orchestration | `app/orchestrator.go` |
| Conversational state management | `core/domain.go` — `State` |
| Agent selection logic | `core/resolver.go` — `shouldEscalate()` |
| Controlled degradation | orchestrator activates escalator only on failure |
| Token budget control | `MaxTokens` per agent in each agent file |
| Provider-agnostic LLM calls | `core/agent.go` — `LLMPort` |
| Structured JSON logging | `slog` throughout, JSON handler in `main.go` |
| Graceful shutdown | `cmd/server/main.go` |

---

## What's next

- [ ] Rate limiting middleware (100 req/min)
- [ ] Request ID middleware for full traceability
- [ ] Sentiment analysis agent
- [ ] Persistent session storage
- [ ] Unit tests with mock LLMPort
- [ ] Voice pipeline integration (bidirectional voice ↔ text)
