# News Routine MCP

`news-routine-mcp` is a Go MCP server version of the upstream [`yencarnacion/news-routine`](https://github.com/yencarnacion/news-routine). Instead of serving a browser dashboard, it exposes the core workflows as MCP tools you can call from Claude/Cowork or any other MCP-compatible client.

## What It Exposes

- `summarize_trade_the_news`
  Converts TradeTheNews text or image attachments into summaries using OpenAI Responses API; supported GPT Audio models accept sound uploads through Chat Completions.
- `run_grok_prompt`
  Runs a current-events prompt against Grok using xAI Responses API with `web_search` and `x_search` enabled by default.
- `run_perplexity_query`
  Runs a Perplexity Sonar query for filings, catalysts, and general research, with optional image attachments on supported models.
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
- Grok defaults to `grok-4.7` with a five-minute timeout. It uses xAI's Responses API with web and X search enabled.
- Perplexity defaults to `sonar-pro`.
- The Marketaux tools preserve the upstream-style defaults for premarket, watchlist, and sector scans.

## Example Tool Calls

TradeTheNews summary:

```json
{
  "email": "FINANCIAL TIMES\n- Stocks rose after...\nBLOOMBERG\n- Oil fell after..."
}
```

OpenAI image summary (`summarize_trade_the_news`):

```json
{
  "image_urls": ["https://example.com/newsletter.png"],
  "prompt_override": "Summarize the market-moving stories in this newsletter."
}
```

The default `gpt-5.6-terra` supports images. For uploads, pass `data:image/png;base64,<base64 file bytes>` (or `image/jpeg`, `image/webp`, `image/gif`) in `image_urls`. GIFs must be non-animated. Multiple images and mixed public HTTP(S) URLs/base64 uploads are supported. This server limits each uploaded image to 20 MiB; OpenAI validates remote files and model-specific limits. Model overrides must support image input on Responses; provider errors are returned for unsupported models. The configured reasoning effort is omitted for GPT-4.1 and GPT-4o models, which do not support that option.

OpenAI sound summary:

```json
{
  "model": "gpt-audio-1.5",
  "audio": [{"format": "wav", "data": "<base64 file bytes>"}],
  "prompt_override": "Summarize the market-moving points in this recording."
}
```

`audio` accepts WAV or MP3 bytes as base64, with a server limit of 25 MiB per clip. Use `gpt-audio-1.5`, `gpt-audio`, or `gpt-audio-mini`; supported snapshots are `gpt-audio-2025-08-28`, `gpt-audio-mini-2025-10-06`, and `gpt-audio-mini-2025-12-15`. These models route to Chat Completions and return text summaries, with reasoning settings omitted. The default Terra model cannot accept audio. Audio models cannot accept images, so mixed image/audio requests are rejected. Other audio models require a capability/routing update before use.

`email` is optional when attachments are present and can supply extra context. The usual news prompt applies unless `prompt_override` is provided. Local paths and audio URLs are not accepted; the MCP client must encode file bytes. Media uploads are validated before any API request, and media requests disable provider response storage. Audio validation checks encoding and file signatures; OpenAI validates the complete recording.

References: [Terra capabilities](https://developers.openai.com/api/docs/models/gpt-5.6-terra), [image inputs](https://developers.openai.com/api/docs/guides/images-vision), [audio inputs](https://developers.openai.com/api/docs/guides/audio-chat-completions).

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

Grok image query (`run_grok_prompt`):

```json
{
  "prompt": "Describe this chart and explain the notable price moves.",
  "image_urls": ["https://example.com/chart.png"],
  "use_web_search": false,
  "use_x_search": false
}
```

For uploads, the MCP client should read the image and pass a data URL in `image_urls`, formatted as `data:image/png;base64,<base64 file bytes>` or `data:image/jpeg;base64,<base64 file bytes>`. Multiple images and mixed URLs/uploads are supported. Local filesystem paths are not accepted. A prompt is still required.

Grok 4.7 accepts JPEG/PNG images up to 20 MiB each. Upload encoding, size, and media type are validated before contacting xAI; remote image contents and limits are validated by xAI. Image requests set `store: false`, as recommended by xAI. Model overrides must support image input when attachments are supplied. Audio uploads are not exposed because Grok 4.7 supports text and image input only; xAI's separate speech APIs are not part of this tool.

References: [Grok 4.7 capabilities](https://docs.x.ai/developers/grok-4-7), [image input format and limits](https://docs.x.ai/developers/model-capabilities/images/understanding).

Grok settings in `config.yaml`:

```yaml
providers:
  grok:
    model: grok-4.7
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

Perplexity image query (`run_perplexity_query`):

```json
{
  "query": "Explain this chart and research the relevant company news.",
  "image_urls": ["https://example.com/chart.png"],
  "search_mode": "web"
}
```

For uploads, the MCP client should pass `data:image/png;base64,<base64 file bytes>` in `image_urls`; JPEG (`image/jpeg`), WebP (`image/webp`), and GIF (`image/gif`) are also accepted. Multiple images and mixed HTTPS URLs/base64 uploads are supported. A `query` is required, and the default model remains `sonar-pro`. Search mode retains the configured default (`sec`) unless overridden, as above.

Image attachments are enabled for `sonar`, `sonar-pro`, and `sonar-reasoning-pro`. Other model selections, including `sonar-deep-research`, are rejected for image requests; text queries retain their existing behavior. Uploaded images are limited to 50 MB (50,000,000 bytes) each and checked for base64 encoding and matching media signatures. Perplexity validates complete files and remote image contents. Remote images must use public HTTPS URLs; local paths are not accepted.

The Sonar API does not document audio inputs, so this tool does not expose sound uploads. Audio data URLs are rejected. The Perplexity consumer app's file/voice features do not establish Sonar API support.

References: [Sonar media inputs](https://docs.perplexity.ai/docs/sonar/media), [Sonar Reasoning Pro](https://docs.perplexity.ai/docs/sonar/models/sonar-reasoning-pro), [Sonar API reference](https://docs.perplexity.ai/api-reference/sonar-post).

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
