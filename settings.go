package main

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type PPLXQuery struct {
	Type        string `yaml:"type" json:"type"`
	Prompt      string `yaml:"prompt" json:"prompt"`
	Placeholder string `yaml:"placeholder" json:"placeholder"`
	Label       string `yaml:"label" json:"label"`
}

type Settings struct {
	NewsPrompt  string      `yaml:"news_prompt" json:"news_prompt"`
	GrokPrompts []string    `yaml:"grok_prompts" json:"grok_prompts"`
	PPLXQueries []PPLXQuery `yaml:"pplx_queries" json:"pplx_queries"`
}

func loadSettings(path string) (Settings, error) {
	defaults := defaultSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return Settings{}, err
	}

	var settings Settings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
	}

	if settings.NewsPrompt == "" {
		settings.NewsPrompt = defaults.NewsPrompt
	}
	if len(settings.GrokPrompts) == 0 {
		settings.GrokPrompts = defaults.GrokPrompts
	}
	if len(settings.PPLXQueries) == 0 {
		settings.PPLXQueries = defaults.PPLXQueries
	}

	return settings, nil
}

func defaultSettings() Settings {
	return Settings{
		NewsPrompt: "There are a number of news sources listed below (like the Financial Times, INVESTORS BUSINESS DAILY, USA Today, etc.).\nUnderneath each source are summarized news headlines that start with a dash (\"-\").\n\nFor each news headline starting with a dash (\"-\"), summarize it as a single sentence, capturing the essence of what was said.\nMaintain the news section headings as shown in the provided format.\nEnsure the final output follows this structure, where the beginning of each news summary has a couple of words in bold, serving as a mini title:\n\n**NEWS SOURCE NAME**\n- **Mini Title:** Headline summary 1\n- **Mini Title:** Headline summary 2\n- **Mini Title:** Headline summary 3\n",
		GrokPrompts: []string{
			"Provide me a digest of world news in the last 24 hours.",
			"what are the key stories and trends from recent sources",
			"what are todays headlines",
			"what are todays business, stock market, and wall street key stories and trends from recent sources",
			"what are todays business, stock market, and wall street headlines",
			"what numbers or economic data or announcements for today are of importance to the us stock market and I should be aware of",
			"what are todays tech and ai and openai key stories and trends from recent sources",
			"what are todays tech and ai and openai headlines",
			"what are todays bitcoin and crypto key stories and trends from recent sources",
			"what are todays bitcoin and crypto headlines",
			"what are the key stories and trends from recent sources from today right now",
			"what are the trending headlines right now",
			"what are todays key stories and trends from recent sources related to puerto rico",
			"what are todays headlines related to puerto rico",
			"create a end-of-the-day summary for the most recent day the market was open based on major financial news, key market movements, and economic indicators",
			"create a end-of-the-week summary for the most recent week the market was open based on major financial news, key market movements, and economic indicators",
			"what numbers or economic data or announcements for next week are of importance to the us stock market and I should be aware of",
			"track the performance of Apple, Tesla, Microsoft, and Nvidia during the most recent trading session, including price change, volume, and key metrics. Also compare the performance of these stocks to the general performance of their respective sectors",
			"Analyze the latest news headlines and social media mentions of Tesla and summarize the public sentiment as positive, negative, or neutral. Provide the most relevant links\n**Extensions:**\n- Break down sentiment classification by media source\n- Identify sentiment of the latest product launches, feature updates, and earnings report, if applicable",
			"what companies have announced earnings since the most recent us markets close. Any surprises. Analyze the latest news headlines and social media mentions of these companies and summarize the public sentiment as positive, negative, or neutral. Provide the most relevant links **Extensions:** - Break down sentiment classification by media source - Identify sentiment of the latest product launches, feature updates, and earnings report, if applicable\n\nFor your answer include a section titled Earnings Details and Surprises\n\ninclude\n**name of company ($TICKER):**\nEarnings: Estimated Earnings of $dollar-amount vs actual earnings of $dollar-amount (difference between EPS and Actual)\nRevenue: Actual Revenue vs Estimated (indicate if beat or missed and by how much)\nSurprise: if any\nStock Movement: Up/Down % and your best guess as to why up/down\n\nAlso have a section titled Analysis of News Headlines and Social Media Mentions\n\ninclude\n**name of company ($TICKER):**\n**News Headlines:**\n**Social Media Mentions:**\n**Public Sentiment:**\n**Sentiment by Media Source:**\n**Web News:**\n**Product Launches/Feature Updates:**\n**Earnings Report Sentiment:**\n",
		},
		PPLXQueries: []PPLXQuery{
			{
				Type:   "fixed",
				Prompt: "Prepare me for when markets open today?",
				Label:  "Prepare me for when markets open today?",
			},
			{
				Type:        "template",
				Prompt:      "Give the main takeaways in markdown about the following: {{context}}. Give your best guess of how the stock will react to this filing from the perspective of a day trader. include the intrument ticker if there is one and you know it.",
				Placeholder: "context",
				Label:       "Filing Takeaways",
			},
			{
				Type:        "template",
				Prompt:      "You are an equity news summarizer for short-term traders.\nINPUT: {{page}} (This may include site chrome, menus, and other noise.)\nDATE CONTEXT: Assume \"the open\" means the next regular U.S. market session after the events in the article.\nTASKS\n1) Parse the article content only (ignore headers, footers, menus, ads). Identify each company mentioned with a clear, company-specific catalyst (earnings, guidance, M&A, regulation, analyst action, operational update, etc.).\n2) For each company, extract:\n   - Company name and instrument ticker (if present or inferable from the text).\n   - The single most important takeaway in 1-2 sentences with key numbers (beats/misses, guidance, deal value, % moves, etc.).\n3) For each company, add a ONE-PARAGRAPH OR SHORTER \"Day-Trader Open Read\":\n   - Give your best guess of how the stock will behave at the next open in trader terms (e.g., \"gap up + possible continuation,\" \"gap up then fade,\" \"gap down continuation,\" \"flat/indecisive\").\n   - Include a one-sentence rationale tied to the catalyst (surprise vs. expectations, quality of guidance, deal math, supply/demand cues).\n   - Keep it concise (<=1-2 sentences). Do NOT give advice or a trade plan; just the likely directional behavior and brief reason.\n   - If the ticker is not in the article and cannot be confidently inferred, write \"Ticker: n/a\".\nOUTPUT FORMAT (Markdown)\n- Start with: `### Main Takeaways`\n- Then, for each company, use exactly this structure:\n- **<Company Name> (<TICKER or n/a>)**: <1-2 sentence key takeaway with numbers>.\n  _Open read:_ <<=1-2 sentence directional guess at the next open (gap/continuation/fade/flat) + rationale>.\nRULES & STYLE\n- Be definitive but realistic; avoid hedging like \"might/maybe\" unless uncertainty is material.\n- Prefer the primary U.S.-listed common ticker when multiple classes exist.\n- If an article shows intraday % moves, you may use them as context but still frame the prediction for the next open.\n- Keep each \"Open read\" to one short paragraph or less.\n- Maximum 100 companies. Skip purely macro notes that don't attach to a specific ticker.",
				Placeholder: "page",
				Label:       "in play stocks",
			},
			{
				Type:  "custom",
				Label: "Custom query",
			},
			{
				Type:        "template",
				Prompt:      "for this conversation i want you to use your expert daytrading, investing and wall street knowledge.\napply the appropriate prompt to the press release below:\n---\n**Prompt:**\nAnalyze the following press release(s) about private placements. Your task is to:\n1. Deal Structure\n   * Summarize the financial terms (amount raised, securities offered, pricing, conversion terms, discounts, warrants, in-kind contributions, etc.).\n   * Note any unusual or complex features (crypto components, reverse splits, excessive dilution, etc.).\n2. Shareholder Impact\n   * Assess the potential dilution impact.\n   * Evaluate whether the capital raised strengthens or weakens the company's balance sheet.\n   * Consider strategic alignment (does this support the core business, or is it a distraction?).\n   * Identify risks (regulatory, execution, speculative assets, etc.).\n3. Comparative Analysis\n   * Benchmark against typical private placement terms in the public markets.\n   * Highlight whether insiders, strategic partners, or credible institutions are involved.\n4. Final Evaluation\n   * Provide a clear judgment: is this private placement good, neutral, or bad for existing shareholders?\n   * Assign a score from -10 to 10, where -10 is the worst possible deal, 0 is neutral, and 10 is the best possible deal.\nThen, provide the final answer twice (once for shareholders and another for stock reaction) in this format:\n**Verdict:** [Good / Neutral / Bad]\n**Score:** [X out of -10 to 10]\n**Reasoning:** [Short, clear justification in plain English]\n---\n**Prompt:**\nAnalyze the following press release about an offering of common stock. Your task is to:\n1. Deal Structure\n   * Summarize the offering size, price per share, number of shares, and gross proceeds.\n   * Note whether the company or selling shareholders are offering the shares.\n   * Identify the underwriters and any over-allotment/greenshoe options.\n2. Shareholder Impact\n   * Assess dilution to existing shareholders (compare offering size to current float/market cap if provided).\n   * Evaluate whether the offering strengthens the balance sheet (cash runway, debt repayment, R&D funding, etc.).\n   * Consider whether the use of proceeds is clear and aligned with the company's core business strategy.\n   * Identify risks: offering price discount, pressure on share price, or repeated capital raises.\n3. Comparative Analysis\n   * Benchmark terms vs. typical underwritten offerings (discount size, underwriter quality, deal size relative to market cap).\n   * Consider whether the offering was done opportunistically (high valuation window) or out of necessity (cash crunch).\n4. Final Evaluation\n   * Provide a clear judgment: good, neutral, or bad for shareholders.\n   * Assign a score from -10 to 10, where -10 is extremely harmful, 0 is neutral, and 10 is extremely beneficial.\nThen, provide the final answer twice (once for shareholders and another for stock reaction) in this format:\n**Verdict:** [Good / Neutral / Bad]\n**Score:** [X out of -10 to 10]\n**Reasoning:** [Short, plain-English justification]\n---\nINPUT: {{page}} (This may include site chrome, menus, and other noise.)",
				Placeholder: "page",
				Label:       "offering or private placement",
			},
		},
	}
}
