package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PromptPresetsResult struct {
	NewsPrompt    string          `json:"news_prompt"`
	GrokPrompts   []string        `json:"grok_prompts"`
	PPLXQueries   []PPLXQuery     `json:"pplx_queries"`
	ModelDefaults ModelDefaults   `json:"model_defaults"`
	Server        ServerReference `json:"server"`
}

type ModelDefaults struct {
	OpenAI     string `json:"openai"`
	Grok       string `json:"grok"`
	Perplexity string `json:"perplexity"`
}

type ServerReference struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Path string `json:"path"`
}

type SummarizeTradeTheNewsInput struct {
	Email           string `json:"email" jsonschema:"Raw TradeTheNews or newsletter text to summarize."`
	PromptOverride  string `json:"prompt_override,omitempty" jsonschema:"Optional full prompt to use instead of settings.yaml news_prompt."`
	Model           string `json:"model,omitempty" jsonschema:"Optional OpenAI model override."`
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema:"Optional OpenAI reasoning effort override."`
}

type GrokPromptInput struct {
	Prompt           string   `json:"prompt" jsonschema:"Prompt to send to Grok."`
	Model            string   `json:"model,omitempty" jsonschema:"Optional Grok model override."`
	UseWebSearch     *bool    `json:"use_web_search,omitempty" jsonschema:"Enable xAI web_search."`
	UseXSearch       *bool    `json:"use_x_search,omitempty" jsonschema:"Enable xAI x_search."`
	AllowedDomains   []string `json:"allowed_domains,omitempty" jsonschema:"Limit web search to these domains."`
	ExcludedDomains  []string `json:"excluded_domains,omitempty" jsonschema:"Exclude these domains from web search."`
	AllowedXHandles  []string `json:"allowed_x_handles,omitempty" jsonschema:"Limit x_search to these X handles."`
	ExcludedXHandles []string `json:"excluded_x_handles,omitempty" jsonschema:"Exclude these X handles from x_search."`
	FromDate         string   `json:"from_date,omitempty" jsonschema:"Optional ISO date for x_search start."`
	ToDate           string   `json:"to_date,omitempty" jsonschema:"Optional ISO date for x_search end."`
}

type PerplexityQueryInput struct {
	Query      string `json:"query" jsonschema:"Prompt to send to Perplexity."`
	Model      string `json:"model,omitempty" jsonschema:"Optional Perplexity model override."`
	SearchMode string `json:"search_mode,omitempty" jsonschema:"Optional Perplexity search mode override."`
}

type MarketauxPremarketInput struct {
	Countries      string `json:"countries,omitempty" jsonschema:"Marketaux countries filter."`
	PublishedAfter string `json:"published_after,omitempty" jsonschema:"ISO datetime. Defaults to today's 09:00 local date string."`
	MinDocCount    int    `json:"min_doc_count,omitempty" jsonschema:"Minimum document count."`
	Limit          int    `json:"limit,omitempty" jsonschema:"Maximum number of rows."`
	Sort           string `json:"sort,omitempty" jsonschema:"Optional sort field."`
	SortOrder      string `json:"sort_order,omitempty" jsonschema:"Optional sort order."`
	GroupBy        string `json:"group_by,omitempty" jsonschema:"Optional grouping key."`
}

type MarketauxWatchlistInput struct {
	Symbols        string `json:"symbols" jsonschema:"Comma-separated ticker list."`
	Interval       string `json:"interval,omitempty" jsonschema:"Marketaux intraday interval."`
	PublishedAfter string `json:"published_after,omitempty" jsonschema:"ISO datetime. Defaults to 90 minutes ago."`
}

type MarketauxSectorInput struct {
	GroupBy        string   `json:"group_by,omitempty" jsonschema:"Marketaux group_by value."`
	Industries     []string `json:"industries,omitempty" jsonschema:"Industries to include."`
	Interval       string   `json:"interval,omitempty" jsonschema:"Marketaux interval."`
	Countries      string   `json:"countries,omitempty" jsonschema:"Country filter."`
	PublishedAfter string   `json:"published_after,omitempty" jsonschema:"ISO datetime. Defaults to 8 hours ago."`
}

func newMCPServer(app *App) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "news-routine-mcp",
		Title:   "News Routine MCP",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: "News Routine MCP exposes tools for TradeTheNews summaries, Grok research prompts, Perplexity research queries, and Marketaux scanners.",
		Logger:       app.Logger,
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_prompt_presets",
		Description: "Return the loaded prompts from settings.yaml plus the current default models from config.yaml.",
	}, app.handleListPromptPresets)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "summarize_trade_the_news",
		Description: "Summarize raw TradeTheNews or newsletter text with OpenAI using the settings.yaml news prompt.",
	}, app.handleSummarizeTradeTheNews)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_grok_prompt",
		Description: "Run a current-events research prompt against Grok using xAI Responses API with web_search and x_search support.",
	}, app.handleRunGrokPrompt)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_perplexity_query",
		Description: "Run a Perplexity Sonar query for filings, catalysts, or general research.",
	}, app.handleRunPerplexityQuery)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketaux_premarket_scan",
		Description: "Fetch the Marketaux premarket aggregation scan used by the original project.",
	}, app.handleMarketauxPremarket)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketaux_watchlist_intraday",
		Description: "Fetch Marketaux intraday stats for a watchlist of symbols.",
	}, app.handleMarketauxWatchlist)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketaux_sector_rotation",
		Description: "Fetch Marketaux sector or industry rotation stats.",
	}, app.handleMarketauxSectorRotation)

	return server
}

func (a *App) handleListPromptPresets(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, PromptPresetsResult, error) {
	out := PromptPresetsResult{
		NewsPrompt:  a.Settings.NewsPrompt,
		GrokPrompts: a.Settings.GrokPrompts,
		PPLXQueries: a.Settings.PPLXQueries,
		ModelDefaults: ModelDefaults{
			OpenAI:     a.Config.Providers.OpenAI.Model,
			Grok:       a.Config.Providers.Grok.Model,
			Perplexity: a.Config.Providers.Perplexity.Model,
		},
		Server: ServerReference{
			Host: a.Config.Server.Host,
			Port: a.Config.Server.Port,
			Path: a.Config.Server.Path,
		},
	}

	return toolResultWithJSON(out), out, nil
}

func (a *App) handleSummarizeTradeTheNews(ctx context.Context, _ *mcp.CallToolRequest, in SummarizeTradeTheNewsInput) (*mcp.CallToolResult, AIResult, error) {
	email := strings.TrimSpace(in.Email)
	if email == "" {
		return nil, AIResult{}, fmt.Errorf("email is required")
	}

	prompt := strings.TrimSpace(in.PromptOverride)
	if prompt == "" {
		prompt = a.Settings.NewsPrompt
	}

	model := fallbackString(in.Model, a.Config.Providers.OpenAI.Model)
	reasoningEffort := fallbackString(in.ReasoningEffort, a.Config.Providers.OpenAI.ReasoningEffort)
	fullPrompt := strings.TrimSpace(prompt) + "\n\n" + email

	result, err := a.callOpenAIResponse(ctx, model, reasoningEffort, fullPrompt)
	if err != nil {
		return nil, AIResult{}, err
	}

	return toolResultWithText(result.Text), result, nil
}

func (a *App) handleRunGrokPrompt(ctx context.Context, _ *mcp.CallToolRequest, in GrokPromptInput) (*mcp.CallToolResult, AIResult, error) {
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return nil, AIResult{}, fmt.Errorf("prompt is required")
	}

	result, err := a.callXAIResponse(ctx, xAIRequest{
		Prompt:           prompt,
		Model:            fallbackString(in.Model, a.Config.Providers.Grok.Model),
		UseWebSearch:     boolOrDefault(in.UseWebSearch, a.Config.Providers.Grok.UseWebSearch),
		UseXSearch:       boolOrDefault(in.UseXSearch, a.Config.Providers.Grok.UseXSearch),
		AllowedDomains:   firstNonEmptySlice(in.AllowedDomains, a.Config.Providers.Grok.AllowedDomains),
		ExcludedDomains:  firstNonEmptySlice(in.ExcludedDomains, a.Config.Providers.Grok.ExcludedDomains),
		AllowedXHandles:  firstNonEmptySlice(in.AllowedXHandles, a.Config.Providers.Grok.AllowedXHandles),
		ExcludedXHandles: firstNonEmptySlice(in.ExcludedXHandles, a.Config.Providers.Grok.ExcludedXHandles),
		FromDate:         fallbackString(in.FromDate, a.Config.Providers.Grok.FromDate),
		ToDate:           fallbackString(in.ToDate, a.Config.Providers.Grok.ToDate),
	})
	if err != nil {
		return nil, AIResult{}, err
	}

	return toolResultWithText(renderAIResult(result)), result, nil
}

func (a *App) handleRunPerplexityQuery(ctx context.Context, _ *mcp.CallToolRequest, in PerplexityQueryInput) (*mcp.CallToolResult, AIResult, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, AIResult{}, fmt.Errorf("query is required")
	}

	result, err := a.callPerplexityResponse(
		ctx,
		fallbackString(in.Model, a.Config.Providers.Perplexity.Model),
		fallbackString(in.SearchMode, a.Config.Providers.Perplexity.SearchMode),
		query,
	)
	if err != nil {
		return nil, AIResult{}, err
	}

	return toolResultWithText(renderAIResult(result)), result, nil
}

func (a *App) handleMarketauxPremarket(ctx context.Context, _ *mcp.CallToolRequest, in MarketauxPremarketInput) (*mcp.CallToolResult, MarketauxResult, error) {
	params := url.Values{}
	params.Set("countries", fallbackString(in.Countries, a.Config.Providers.Marketaux.PremarketCountries))
	params.Set("published_after", fallbackString(in.PublishedAfter, time.Now().Format("2006-01-02")+"T09:00:00"))
	params.Set("min_doc_count", fmt.Sprintf("%d", fallbackInt(in.MinDocCount, a.Config.Providers.Marketaux.PremarketMinDoc)))
	params.Set("limit", fmt.Sprintf("%d", fallbackInt(in.Limit, a.Config.Providers.Marketaux.PremarketLimit)))
	if strings.TrimSpace(in.Sort) != "" {
		params.Set("sort", strings.TrimSpace(in.Sort))
	}
	if strings.TrimSpace(in.SortOrder) != "" {
		params.Set("sort_order", strings.TrimSpace(in.SortOrder))
	}
	if strings.TrimSpace(in.GroupBy) != "" {
		params.Set("group_by", strings.TrimSpace(in.GroupBy))
	}

	result, err := a.callMarketaux(ctx, "/stats/aggregation", params)
	if err != nil {
		return nil, MarketauxResult{}, err
	}

	return toolResultWithJSON(result), result, nil
}

func (a *App) handleMarketauxWatchlist(ctx context.Context, _ *mcp.CallToolRequest, in MarketauxWatchlistInput) (*mcp.CallToolResult, MarketauxResult, error) {
	symbols := strings.TrimSpace(in.Symbols)
	if symbols == "" {
		return nil, MarketauxResult{}, fmt.Errorf("symbols is required")
	}

	params := url.Values{}
	params.Set("symbols", symbols)
	params.Set("interval", fallbackString(in.Interval, a.Config.Providers.Marketaux.WatchlistInterval))
	params.Set("published_after", fallbackString(in.PublishedAfter, time.Now().Add(-90*time.Minute).Format("2006-01-02T15:04:05")))

	result, err := a.callMarketaux(ctx, "/stats/intraday", params)
	if err != nil {
		return nil, MarketauxResult{}, err
	}

	return toolResultWithJSON(result), result, nil
}

func (a *App) handleMarketauxSectorRotation(ctx context.Context, _ *mcp.CallToolRequest, in MarketauxSectorInput) (*mcp.CallToolResult, MarketauxResult, error) {
	params := url.Values{}
	params.Set("group_by", fallbackString(in.GroupBy, "industry"))
	params.Set("interval", fallbackString(in.Interval, a.Config.Providers.Marketaux.SectorInterval))
	params.Set("countries", fallbackString(in.Countries, "us"))
	params.Set("published_after", fallbackString(in.PublishedAfter, time.Now().Add(-8*time.Hour).Format("2006-01-02T15:04:05")))
	params.Set("industries", strings.Join(firstNonEmptySlice(in.Industries, a.Config.Providers.Marketaux.SectorIndustries), ","))

	result, err := a.callMarketaux(ctx, "/stats/intraday", params)
	if err != nil {
		return nil, MarketauxResult{}, err
	}

	return toolResultWithJSON(result), result, nil
}

func renderAIResult(result AIResult) string {
	if len(result.Citations) == 0 {
		return result.Text
	}
	return result.Text + "\n\nSources:\n- " + strings.Join(result.Citations, "\n- ")
}

func toolResultWithText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func toolResultWithJSON(v any) *mcp.CallToolResult {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		body = []byte("{}")
	}
	return toolResultWithText(string(body))
}

func boolOrDefault(value *bool, d bool) bool {
	if value == nil {
		return d
	}
	return *value
}

func firstNonEmptySlice(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}
