# News Routine MCP

`news-routine-mcp` is a Go MCP server version of the upstream [`yencarnacion/news-routine`](https://github.com/yencarnacion/news-routine). Instead of serving a browser dashboard, it exposes the core workflows as MCP tools you can call from Claude/Cowork or any other MCP-compatible client.

## What It Exposes

- `summarize_trade_the_news`
  Converts pasted TradeTheNews or newsletter text into structured summaries using OpenAI Responses API.
- `run_grok_prompt`
  Runs a current-events prompt against Grok using xAI Responses API with `web_search` and `x_search` enabled by default.
- `run_perplexity_query`
  Runs a Perplexity Sonar query for filings, catalysts, and general research.
- `marketaux_premarket_scan`
  Returns the Marketaux premarket aggregation scan.
- `marketaux_watchlist_intraday`
  Returns Marketaux intraday stats for a watchlist.
- `marketaux_sector_rotation`
  Returns Marketaux sector or industry rotation stats.
- `list_prompt_presets`
  Returns the loaded `settings.yaml` prompts plus the current model defaults from `config.yaml`.

## Files

- `config.yaml`
  Runtime config for the MCP server, including default host, port, path, and provider defaults.
- `settings.yaml`
  Prompt presets ported from the upstream repo.
- `env.example`
  Template for `.env`.
- `assumptions.md`
  Important interpretation decisions made during the port.

## Requirements

- Go 1.25 or newer
- API keys for the providers you plan to use

## Setup

```bash
cp env.example .env
```

Fill in whichever keys you need:

```dotenv
OPENAI_API_KEY=...
GROK_API_KEY=...
PPLX_API_KEY=...
MARKETAUX_API_KEY=...
```

The defaults live in `config.yaml`. By default the MCP endpoint is:

```text
http://127.0.0.1:9081/mcp
```

## Run

Streamable HTTP mode:

```bash
go run .
```

Explicit HTTP mode:

```bash
go run . serve
```

Stdio mode:

```bash
go run . stdio
```

Alternate files:

```bash
go run . --config ./config.yaml --env-file ./.env
```

## Claude / Cowork Usage

I interpreted "Claude Cowork" as a Claude-family MCP client. If your client supports Streamable HTTP MCP servers, point it at the default local URL.

Example Streamable HTTP config:

```json
{
  "mcpServers": {
    "news-routine": {
      "url": "http://127.0.0.1:9081/mcp"
    }
  }
}
```

If your Claude client prefers launching a local command over stdio, use:

```json
{
  "mcpServers": {
    "news-routine": {
      "command": "go",
      "args": [
        "run",
        ".",
        "stdio"
      ],
      "cwd": "/home/yamir/Documents/news-routine-mcp"
    }
  }
}
```

## Notes on Provider Defaults

- OpenAI defaults to `gpt-5.6-terra` with `high` reasoning effort.
- Grok defaults to `grok-4.6` with a five-minute timeout. It uses xAI's Responses API with web and X search enabled.
- Perplexity defaults to `sonar-pro`.
- The Marketaux tools preserve the upstream-style defaults for premarket, watchlist, and sector scans.

## Example Tool Calls

TradeTheNews summary:

```json
{
  "email": "FINANCIAL TIMES\n- Stocks rose after...\nBLOOMBERG\n- Oil fell after..."
}
```

Grok current-events query:

```json
{
  "prompt": "What are today's most important U.S. market-moving stories?",
  "reasoning_effort": "medium",
  "prompt_cache_key": "morning-news",
  "use_web_search": true,
  "use_x_search": true
}
```

Grok settings in `config.yaml`:

```yaml
providers:
  grok:
    model: grok-4.6
    reasoning_effort: high
    prompt_cache_key: news-routine-mcp
```

`reasoning_effort` accepts `low`, `medium`, `high`, or `xhigh`. Omitted or blank effort uses the configured default; invalid values are rejected before an API request. Lower effort is useful for quick scans; `xhigh` can take longer and consume more reasoning tokens. The existing five-minute timeout still applies.

`prompt_cache_key` is a stable routing hint for related requests. The default is `news-routine-mcp`; use a separate key per workflow or conversation when appropriate. Omitting the tool argument inherits the YAML value. An explicit empty string (in YAML or a tool call) omits the routing hint; it does not disable automatic provider caching. A key does not preserve conversation history or reuse an old answer.

The server keeps its fixed system instructions before the variable user prompt. Put reusable background first and changing questions last in your prompts to preserve matching prefixes. Cache hits are not guaranteed, especially for short prompts or requests far apart.

Grok results include `usage` in structured output and a token usage footer in text, when xAI supplies usage. It reports input, cached input, output, reasoning, and total tokens. Cached tokens are part of input tokens, and reasoning tokens are part of output tokens; do not add them again. Missing usage is omitted, not reported as a cache miss. Token counts let you measure cache reuse; they are not a dollar-cost estimate.

References: [reasoning](https://docs.x.ai/developers/model-capabilities/text/reasoning), [cache routing](https://docs.x.ai/developers/advanced-api-usage/prompt-caching/maximizing-cache-hits), [usage accounting](https://docs.x.ai/developers/advanced-api-usage/prompt-caching/usage-and-pricing).

Perplexity filing query:

```json
{
  "query": "Summarize the latest 8-K filing for NVDA and tell me the main day-trader takeaway."
}
```

Marketaux watchlist:

```json
{
  "symbols": "NVDA,TSLA,MSFT"
}
```

## Verification

Build:

```bash
go build ./...
```

Test:

```bash
go test ./...
```

## Upstream Behavior Preserved

- `.env`-based API key loading
- YAML-based prompt presets
- OpenAI summary workflow
- Grok research workflow
- Perplexity workflow
- Marketaux scanners

## References

- OpenAI Responses API: https://platform.openai.com/docs/api-reference/responses/create?api-mode=responses
- OpenAI model comparison: https://developers.openai.com/api/docs/models/compare
- xAI Web Search tool: https://docs.x.ai/developers/tools/web-search
- xAI X Search tool: https://docs.x.ai/developers/tools/x-search
- Perplexity Chat Completions quickstart: https://docs.perplexity.ai/docs/grounded-llm/chat-completions/quickstart
- Marketaux API docs: https://www.marketaux.com/documentation
