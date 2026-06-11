package websearch

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"oblivious/server/internal/mcp"
)

// ---------------------------------------------------------------------------
// Tavily — https://docs.tavily.com (POST /search, Bearer auth)
// ---------------------------------------------------------------------------

type Tavily struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewTavily(apiKey, endpoint string, client *http.Client) (*Tavily, error) {
	key, err := requireKey("tavily", apiKey)
	if err != nil {
		return nil, err
	}
	return &Tavily{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://api.tavily.com/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Tavily) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	err = postJSON(ctx, p.client, p.endpoint,
		map[string]string{"Authorization": "Bearer " + p.apiKey},
		map[string]any{"query": query, "max_results": 10},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Results))
	for _, item := range response.Results {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Content})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Brave — https://api.search.brave.com (GET, X-Subscription-Token)
// ---------------------------------------------------------------------------

type Brave struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewBrave(apiKey, endpoint string, client *http.Client) (*Brave, error) {
	key, err := requireKey("brave", apiKey)
	if err != nil {
		return nil, err
	}
	return &Brave{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://api.search.brave.com/res/v1/web/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Brave) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	requestURL := p.endpoint + "?q=" + url.QueryEscape(query) + "&count=10"
	err = getJSON(ctx, p.client, requestURL,
		map[string]string{"X-Subscription-Token": p.apiKey},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Web.Results))
	for _, item := range response.Web.Results {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Description})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Serper — https://serper.dev (POST, X-API-KEY)
// ---------------------------------------------------------------------------

type Serper struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewSerper(apiKey, endpoint string, client *http.Client) (*Serper, error) {
	key, err := requireKey("serper", apiKey)
	if err != nil {
		return nil, err
	}
	return &Serper{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://google.serper.dev/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Serper) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}
	err = postJSON(ctx, p.client, p.endpoint,
		map[string]string{"X-API-KEY": p.apiKey},
		map[string]any{"q": query, "num": 10},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Organic))
	for _, item := range response.Organic {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.Link, Snippet: item.Snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// SerpAPI — https://serpapi.com (GET /search.json?api_key=)
// ---------------------------------------------------------------------------

type SerpAPI struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewSerpAPI(apiKey, endpoint string, client *http.Client) (*SerpAPI, error) {
	key, err := requireKey("serpapi", apiKey)
	if err != nil {
		return nil, err
	}
	return &SerpAPI{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://serpapi.com/search.json"),
		client:   defaultClient(client),
	}, nil
}

func (p *SerpAPI) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}
	requestURL := p.endpoint + "?engine=google&q=" + url.QueryEscape(query) + "&api_key=" + url.QueryEscape(p.apiKey)
	if err := getJSON(ctx, p.client, requestURL, nil, &response); err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.OrganicResults))
	for _, item := range response.OrganicResults {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.Link, Snippet: item.Snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Bing — https://api.bing.microsoft.com (GET, Ocp-Apim-Subscription-Key)
// ---------------------------------------------------------------------------

type Bing struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewBing(apiKey, endpoint string, client *http.Client) (*Bing, error) {
	key, err := requireKey("bing", apiKey)
	if err != nil {
		return nil, err
	}
	return &Bing{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://api.bing.microsoft.com/v7.0/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Bing) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}
	requestURL := p.endpoint + "?q=" + url.QueryEscape(query) + "&count=10"
	err = getJSON(ctx, p.client, requestURL,
		map[string]string{"Ocp-Apim-Subscription-Key": p.apiKey},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.WebPages.Value))
	for _, item := range response.WebPages.Value {
		results = append(results, mcp.WebSearchResult{Title: item.Name, URL: item.URL, Snippet: item.Snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Google Programmable Search — https://developers.google.com/custom-search
// ---------------------------------------------------------------------------

type GoogleCSE struct {
	apiKey   string
	cseID    string
	endpoint string
	client   *http.Client
}

func NewGoogleCSE(apiKey, cseID, endpoint string, client *http.Client) (*GoogleCSE, error) {
	key, err := requireKey("google_cse", apiKey)
	if err != nil {
		return nil, err
	}
	cseID = strings.TrimSpace(cseID)
	if cseID == "" {
		return nil, errors.New("websearch: google_cse requires a search engine ID (cx)")
	}
	return &GoogleCSE{
		apiKey:   key,
		cseID:    cseID,
		endpoint: pickEndpoint(endpoint, "https://www.googleapis.com/customsearch/v1"),
		client:   defaultClient(client),
	}, nil
}

func (p *GoogleCSE) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	requestURL := p.endpoint + "?key=" + url.QueryEscape(p.apiKey) + "&cx=" + url.QueryEscape(p.cseID) + "&q=" + url.QueryEscape(query)
	if err := getJSON(ctx, p.client, requestURL, nil, &response); err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Items))
	for _, item := range response.Items {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.Link, Snippet: item.Snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// DuckDuckGo Instant Answer — https://api.duckduckgo.com (keyless)
// ---------------------------------------------------------------------------

type DuckDuckGo struct {
	endpoint string
	client   *http.Client
}

func NewDuckDuckGo(endpoint string, client *http.Client) (*DuckDuckGo, error) {
	return &DuckDuckGo{
		endpoint: pickEndpoint(endpoint, "https://api.duckduckgo.com"),
		client:   defaultClient(client),
	}, nil
}

type duckDuckGoTopic struct {
	Text     string            `json:"Text"`
	FirstURL string            `json:"FirstURL"`
	Topics   []duckDuckGoTopic `json:"Topics"`
}

func flattenDuckDuckGoTopics(topics []duckDuckGoTopic, out *[]mcp.WebSearchResult) {
	for _, topic := range topics {
		if len(topic.Topics) > 0 {
			flattenDuckDuckGoTopics(topic.Topics, out)
			continue
		}
		if strings.TrimSpace(topic.Text) == "" || strings.TrimSpace(topic.FirstURL) == "" {
			continue
		}
		*out = append(*out, mcp.WebSearchResult{Title: topic.Text, URL: topic.FirstURL, Snippet: topic.Text})
	}
}

func (p *DuckDuckGo) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Heading       string            `json:"Heading"`
		AbstractText  string            `json:"AbstractText"`
		AbstractURL   string            `json:"AbstractURL"`
		RelatedTopics []duckDuckGoTopic `json:"RelatedTopics"`
	}
	requestURL := p.endpoint + "/?q=" + url.QueryEscape(query) + "&format=json&no_html=1&no_redirect=1"
	if err := getJSON(ctx, p.client, requestURL, nil, &response); err != nil {
		return nil, err
	}
	var results []mcp.WebSearchResult
	if strings.TrimSpace(response.AbstractText) != "" && strings.TrimSpace(response.AbstractURL) != "" {
		title := response.Heading
		if strings.TrimSpace(title) == "" {
			title = query
		}
		results = append(results, mcp.WebSearchResult{Title: title, URL: response.AbstractURL, Snippet: response.AbstractText})
	}
	flattenDuckDuckGoTopics(response.RelatedTopics, &results)
	if len(results) > 10 {
		results = results[:10]
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// SearXNG — self-hosted metasearch (GET /search?format=json)
// ---------------------------------------------------------------------------

type SearXNG struct {
	endpoint string
	client   *http.Client
}

func NewSearXNG(endpoint string, client *http.Client) (*SearXNG, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, errors.New("websearch: searxng requires an endpoint (self-hosted instance URL)")
	}
	return &SearXNG{
		endpoint: strings.TrimRight(endpoint, "/"),
		client:   defaultClient(client),
	}, nil
}

func (p *SearXNG) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	requestURL := p.endpoint + "/search?q=" + url.QueryEscape(query) + "&format=json"
	if err := getJSON(ctx, p.client, requestURL, nil, &response); err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Results))
	for _, item := range response.Results {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Content})
	}
	if len(results) > 10 {
		results = results[:10]
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Exa — https://docs.exa.ai (POST /search, x-api-key)
// ---------------------------------------------------------------------------

type Exa struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewExa(apiKey, endpoint string, client *http.Client) (*Exa, error) {
	key, err := requireKey("exa", apiKey)
	if err != nil {
		return nil, err
	}
	return &Exa{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://api.exa.ai/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Exa) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Text  string `json:"text"`
		} `json:"results"`
	}
	err = postJSON(ctx, p.client, p.endpoint,
		map[string]string{"x-api-key": p.apiKey},
		map[string]any{"query": query, "numResults": 10, "contents": map[string]any{"text": true}},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Results))
	for _, item := range response.Results {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: truncateForError(item.Text, 500)})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// You.com — https://api.ydc-index.io (GET, X-API-Key)
// ---------------------------------------------------------------------------

type You struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewYou(apiKey, endpoint string, client *http.Client) (*You, error) {
	key, err := requireKey("you", apiKey)
	if err != nil {
		return nil, err
	}
	return &You{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://api.ydc-index.io/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *You) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Hits []struct {
			Title       string   `json:"title"`
			URL         string   `json:"url"`
			Description string   `json:"description"`
			Snippets    []string `json:"snippets"`
		} `json:"hits"`
	}
	requestURL := p.endpoint + "?query=" + url.QueryEscape(query)
	err = getJSON(ctx, p.client, requestURL,
		map[string]string{"X-API-Key": p.apiKey},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Hits))
	for _, hit := range response.Hits {
		snippet := hit.Description
		if snippet == "" && len(hit.Snippets) > 0 {
			snippet = hit.Snippets[0]
		}
		results = append(results, mcp.WebSearchResult{Title: hit.Title, URL: hit.URL, Snippet: snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Kagi — https://help.kagi.com/kagi/api/search.html (GET, "Bot" auth)
// ---------------------------------------------------------------------------

type Kagi struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewKagi(apiKey, endpoint string, client *http.Client) (*Kagi, error) {
	key, err := requireKey("kagi", apiKey)
	if err != nil {
		return nil, err
	}
	return &Kagi{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://kagi.com/api/v0/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Kagi) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			Type    int    `json:"t"`
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
		} `json:"data"`
	}
	requestURL := p.endpoint + "?q=" + url.QueryEscape(query)
	err = getJSON(ctx, p.client, requestURL,
		map[string]string{"Authorization": "Bot " + p.apiKey},
		&response,
	)
	if err != nil {
		return nil, err
	}
	var results []mcp.WebSearchResult
	for _, item := range response.Data {
		if item.Type != 0 {
			continue
		}
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Mojeek — https://www.mojeek.com/services/search/web-search-api/
// ---------------------------------------------------------------------------

type Mojeek struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewMojeek(apiKey, endpoint string, client *http.Client) (*Mojeek, error) {
	key, err := requireKey("mojeek", apiKey)
	if err != nil {
		return nil, err
	}
	return &Mojeek{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://api.mojeek.com/search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Mojeek) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Response struct {
			Results []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				Desc  string `json:"desc"`
			} `json:"results"`
		} `json:"response"`
	}
	requestURL := p.endpoint + "?q=" + url.QueryEscape(query) + "&api_key=" + url.QueryEscape(p.apiKey) + "&fmt=json"
	if err := getJSON(ctx, p.client, requestURL, nil, &response); err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Response.Results))
	for _, item := range response.Response.Results {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Desc})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Jina — https://jina.ai (GET s.jina.ai, Bearer auth, JSON via Accept)
// ---------------------------------------------------------------------------

type Jina struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewJina(apiKey, endpoint string, client *http.Client) (*Jina, error) {
	key, err := requireKey("jina", apiKey)
	if err != nil {
		return nil, err
	}
	return &Jina{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://s.jina.ai"),
		client:   defaultClient(client),
	}, nil
}

func (p *Jina) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Content     string `json:"content"`
		} `json:"data"`
	}
	requestURL := p.endpoint + "/?q=" + url.QueryEscape(query)
	err = getJSON(ctx, p.client, requestURL,
		map[string]string{"Authorization": "Bearer " + p.apiKey, "X-Respond-With": "no-content"},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Data))
	for _, item := range response.Data {
		snippet := item.Description
		if snippet == "" {
			snippet = truncateForError(item.Content, 500)
		}
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Bocha — https://open.bochaai.com (POST /v1/web-search, Bearer auth)
// ---------------------------------------------------------------------------

type Bocha struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewBocha(apiKey, endpoint string, client *http.Client) (*Bocha, error) {
	key, err := requireKey("bocha", apiKey)
	if err != nil {
		return nil, err
	}
	return &Bocha{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://api.bochaai.com/v1/web-search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Bocha) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data struct {
			WebPages struct {
				Value []struct {
					Name    string `json:"name"`
					URL     string `json:"url"`
					Snippet string `json:"snippet"`
				} `json:"value"`
			} `json:"webPages"`
		} `json:"data"`
	}
	err = postJSON(ctx, p.client, p.endpoint,
		map[string]string{"Authorization": "Bearer " + p.apiKey},
		map[string]any{"query": query, "count": 10, "summary": false},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.Data.WebPages.Value))
	for _, item := range response.Data.WebPages.Value {
		results = append(results, mcp.WebSearchResult{Title: item.Name, URL: item.URL, Snippet: item.Snippet})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Baidu (Qianfan AI Search) — POST /v2/ai_search, Bearer auth
// ---------------------------------------------------------------------------

type Baidu struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewBaidu(apiKey, endpoint string, client *http.Client) (*Baidu, error) {
	key, err := requireKey("baidu", apiKey)
	if err != nil {
		return nil, err
	}
	return &Baidu{
		apiKey:   key,
		endpoint: pickEndpoint(endpoint, "https://qianfan.baidubce.com/v2/ai_search"),
		client:   defaultClient(client),
	}, nil
}

func (p *Baidu) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	query, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	var response struct {
		References []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"references"`
	}
	err = postJSON(ctx, p.client, p.endpoint,
		map[string]string{"Authorization": "Bearer " + p.apiKey},
		map[string]any{
			"messages": []map[string]any{{"role": "user", "content": query}},
		},
		&response,
	)
	if err != nil {
		return nil, err
	}
	results := make([]mcp.WebSearchResult, 0, len(response.References))
	for _, item := range response.References {
		results = append(results, mcp.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: truncateForError(item.Content, 500)})
	}
	return results, nil
}
