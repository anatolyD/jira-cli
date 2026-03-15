package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// SearchResult struct holds response from /search endpoint.
type SearchResult struct {
	IsLast        bool     `json:"isLast"`
	NextPageToken string   `json:"nextPageToken"`
	Issues        []*Issue `json:"issues"`
}

// Search searches for issues using v3 version of the Jira GET /search endpoint.
// Returns a single page of results.
func (c *Client) Search(jql string, limit uint) (*SearchResult, error) {
	path := fmt.Sprintf("/search/jql?jql=%s&maxResults=%d&fields=*all", url.QueryEscape(jql), limit)
	return c.search(path, apiVersion3)
}

// SearchAll searches for issues using v3 version of the Jira GET /search endpoint,
// automatically paginating through all pages using cursor-based pagination (nextPageToken).
func (c *Client) SearchAll(jql string, limit uint) (*SearchResult, error) {
	var allIssues []*Issue

	pageToken := ""
	for {
		path := fmt.Sprintf("/search/jql?jql=%s&maxResults=%d&fields=*all", url.QueryEscape(jql), limit)
		if pageToken != "" {
			path += fmt.Sprintf("&nextPageToken=%s", url.QueryEscape(pageToken))
		}

		result, err := c.search(path, apiVersion3)
		if err != nil {
			return nil, err
		}

		allIssues = append(allIssues, result.Issues...)

		if result.IsLast || result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return &SearchResult{
		IsLast: true,
		Issues: allIssues,
	}, nil
}

// SearchV2 searches an issues using v2 version of the Jira GET /search endpoint.
func (c *Client) SearchV2(jql string, from, limit uint) (*SearchResult, error) {
	path := fmt.Sprintf("/search?jql=%s&startAt=%d&maxResults=%d", url.QueryEscape(jql), from, limit)
	return c.search(path, apiVersion2)
}

// Filter holds saved Jira filter info.
type Filter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	JQL  string `json:"jql"`
}

// FilterSearchResult holds response from /filter/search endpoint.
type FilterSearchResult struct {
	Total   int       `json:"total"`
	IsLast  bool      `json:"isLast"`
	Filters []*Filter `json:"values"`
}

// FilterSearch searches for saved Jira filters by name using GET /rest/api/3/filter/search.
func (c *Client) FilterSearch(name string) (*FilterSearchResult, error) {
	path := fmt.Sprintf("/filter/search?filterName=%s&expand=jql&maxResults=50", url.QueryEscape(name))

	res, err := c.Get(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out FilterSearchResult
	err = json.NewDecoder(res.Body).Decode(&out)
	return &out, err
}

func (c *Client) search(path, ver string) (*SearchResult, error) {
	var (
		res *http.Response
		err error
	)

	switch ver {
	case apiVersion2:
		res, err = c.GetV2(context.Background(), path, nil)
	default:
		res, err = c.Get(context.Background(), path, nil)
	}

	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out SearchResult

	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}
