# Assumptions

- I treated "convert news-routine into an MCP server" as a transport conversion, not a request to preserve the browser dashboard. The result is a Go MCP server that exposes the original workflows as MCP tools instead of HTTP UI tabs.
- I treated "run it from the command line like the repo" as `go run .` starting the server directly, with `go run . stdio` added for MCP clients that prefer stdio instead of HTTP.
- I treated "default to port 9081 configurable in a yaml" as a Streamable HTTP MCP endpoint on `http://127.0.0.1:9081/mcp`, with host, port, path, model defaults, and provider behavior controlled by `config.yaml`.
- I treated "api keys like news-routine from .env" as loading `.env` automatically if present, while still honoring already-exported environment variables.
- I interpreted "Grok default to model 4.2" as the current xAI 4.20 family documented on March 30, 2026. The implementation defaults to `grok-4.20-reasoning` because current xAI tool-use examples use that model name rather than a literal `grok-4.2`.
- I interpreted "search/news/x access" for Grok as enabling xAI Responses API tool use with both `web_search` and `x_search` by default. The current xAI docs expose those tool types instead of a separate `news_search` tool.
- I treated "update the grok code to the latest documentation" as migrating away from the legacy Chat Completions plus `search_parameters` approach in the upstream project to the current xAI Responses API tool model.
- I treated "tradethenews tab" as the upstream "generate summaries from pasted newsletter text" workflow and switched that path to OpenAI Responses API with `gpt-5.6-terra` and `high` reasoning effort as the defaults.
- I interpreted "README explaining how to use from claude cowork" as "Claude-family MCP client usage." The README includes both Streamable HTTP and stdio configuration examples and calls out this interpretation explicitly.
