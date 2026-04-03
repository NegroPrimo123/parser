package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"hh-parser/internal/models"
	"hh-parser/pkg/logger"
	"hh-parser/pkg/metrics"
	"hh-parser/pkg/retry"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

var (
	htmlCleaner = bluemonday.NewPolicy()
	_           = htmlCleaner.AllowElements("p", "br", "li", "ul")
)

type HHParser struct {
	client     *http.Client
	baseURL    string
	userAgents []string
	retryCfg   *retry.Config
}

func (p *HHParser) cleanHTMLText(text string) string {
	// Декодируем HTML сущности
	text = html.UnescapeString(text)

	// Удаляем HTML теги
	text = htmlCleaner.Sanitize(text)

	// Заменяем множественные пробелы и переносы строк
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	// Обрезаем длинные тексты
	if len(text) > 10000 {
		text = text[:10000] + "..."
	}

	return strings.TrimSpace(text)
}

func NewHHParser() *HHParser {
	return &HHParser{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: "https://api.hh.ru",
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/119.0",
		},
		retryCfg: retry.DefaultConfig(),
	}
}

func NewHHParserWithClient(client *http.Client) *HHParser {
	return &HHParser{
		client:  client,
		baseURL: "https://api.hh.ru",
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/119.0",
		},
		retryCfg: retry.DefaultConfig(),
	}
}

func (p *HHParser) SetRetryConfig(cfg *retry.Config) {
	p.retryCfg = cfg
}

func (p *HHParser) getRandomUserAgent() string {
	return p.userAgents[rand.Intn(len(p.userAgents))]
}

type HHSearchResponse struct {
	Items []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		URL    string `json:"alternate_url"`
		Salary *struct {
			From     *int   `json:"from"`
			To       *int   `json:"to"`
			Currency string `json:"currency"`
		} `json:"salary"`
		Employer struct {
			Name string `json:"name"`
		} `json:"employer"`
		Area struct {
			Name string `json:"name"`
		} `json:"area"`
	} `json:"items"`
	Found int `json:"found"`
	Pages int `json:"pages"`
	Page  int `json:"page"`
}

type HHVacancyResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	KeySkills   []struct {
		Name string `json:"name"`
	} `json:"key_skills"`
	AlternateURL string `json:"alternate_url"`
}

func (p *HHParser) SearchVacancies(ctx context.Context, query string, page, perPage int) (interface{}, error) {
	encodedQuery := strings.ReplaceAll(query, " ", "+")
	url := fmt.Sprintf("%s/vacancies?text=%s&page=%d&per_page=%d&area=113",
		p.baseURL, encodedQuery, page, perPage)

	var result *HHSearchResponse
	var statusCode int

	err := retry.Do(ctx, p.retryCfg, func() error {
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			metrics.HTTPRequests.WithLabelValues("GET", "search", fmt.Sprintf("%d", statusCode)).Inc()
			metrics.RequestDuration.WithLabelValues("GET", "search").Observe(duration)
		}()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", p.getRandomUserAgent())
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			statusCode = 0
			return err
		}
		defer resp.Body.Close()

		statusCode = resp.StatusCode

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API returned status: %d, body: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var searchResp HHSearchResponse
		if err := json.Unmarshal(body, &searchResp); err != nil {
			return err
		}

		result = &searchResp
		return nil
	})

	if err != nil {
		logger.Log.Error("Failed to search vacancies",
			"query", query,
			"page", page,
			"error", err,
		)
		return nil, err
	}

	logger.Log.Debug("Search completed",
		"query", query,
		"page", page,
		"found", len(result.Items),
	)

	return result, nil
}

func (p *HHParser) GetVacancyDetails(ctx context.Context, id string) (*models.Vacancy, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	url := fmt.Sprintf("%s/vacancies/%s", p.baseURL, id)

	var apiResp HHVacancyResponse
	var statusCode int

	err := retry.Do(ctx, p.retryCfg, func() error {
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			metrics.HTTPRequests.WithLabelValues("GET", "details", fmt.Sprintf("%d", statusCode)).Inc()
			metrics.RequestDuration.WithLabelValues("GET", "details").Observe(duration)
		}()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", p.getRandomUserAgent())
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			statusCode = 0
			return err
		}
		defer resp.Body.Close()

		statusCode = resp.StatusCode

		if resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("access denied (403), need captcha or rate limit exceeded")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API returned status: %d, body: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(body, &apiResp); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		logger.Log.Warn("Failed to get vacancy details",
			"vacancy_id", id,
			"error", err,
		)
		return nil, err
	}

	cleanText := func(text string) string {
		text = html.UnescapeString(text)
		text = strings.ReplaceAll(text, "<p>", " ")
		text = strings.ReplaceAll(text, "</p>", " ")
		text = strings.ReplaceAll(text, "<br>", " ")
		text = strings.ReplaceAll(text, "<br/>", " ")
		text = strings.ReplaceAll(text, "<li>", "- ")
		text = strings.ReplaceAll(text, "</li>", " ")
		text = strings.ReplaceAll(text, "<ul>", " ")
		text = strings.ReplaceAll(text, "</ul>", " ")
		text = strings.ReplaceAll(text, "&nbsp;", " ")
		text = strings.ReplaceAll(text, "&amp;", "&")
		text = strings.ReplaceAll(text, "&lt;", "<")
		text = strings.ReplaceAll(text, "&gt;", ">")
		text = strings.Join(strings.Fields(text), " ")
		if len(text) > 10000 {
			text = text[:10000]
		}
		return text
	}

	vacancy := &models.Vacancy{
		HHID:        apiResp.ID,
		Name:        cleanText(apiResp.Name),
		URL:         apiResp.AlternateURL,
		Requirement: cleanText(apiResp.Description),
		CreatedAt:   time.Now(),
	}

	logger.Log.Debug("Vacancy details fetched", "id", id, "name", vacancy.Name)

	return vacancy, nil
}
