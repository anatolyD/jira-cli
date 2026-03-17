package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ChecklistItem represents a single checklist item.
type ChecklistItem struct {
	Text   string
	Status string // "open" or "done"
}

// GetChecklist retrieves checklist items from the specified custom field on an issue.
func (c *Client) GetChecklist(key, customField string) ([]ChecklistItem, error) {
	path := fmt.Sprintf("/issue/%s?fields=%s", key, customField)

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

	var raw struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}

	fieldData, ok := raw.Fields[customField]
	if !ok || string(fieldData) == "null" {
		return nil, nil
	}

	return parseChecklistADF(fieldData)
}

// SetChecklist writes checklist items to the specified custom field on an issue.
func (c *Client) SetChecklist(key, customField string, items []ChecklistItem) error {
	adf := buildChecklistADF(items)

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			customField: adf,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	res, err := c.Put(context.Background(), "/issue/"+key, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return err
	}
	if res == nil {
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		return formatUnexpectedResponse(res)
	}
	return nil
}

// parseChecklistADF parses the ADF document format used by the Checklist plugin.
func parseChecklistADF(data json.RawMessage) ([]ChecklistItem, error) {
	var doc struct {
		Content []struct {
			Type    string `json:"type"`
			Content []struct {
				Content []struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"content"`
			} `json:"content"`
		} `json:"content"`
	}

	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse checklist ADF: %w", err)
	}

	var items []ChecklistItem
	for _, block := range doc.Content {
		if block.Type != "bulletList" {
			continue
		}
		for _, listItem := range block.Content {
			for _, para := range listItem.Content {
				for _, text := range para.Content {
					item := parseChecklistItemText(text.Text)
					items = append(items, item)
				}
			}
		}
	}

	return items, nil
}

// parseChecklistItemText parses "[open] text" or "[done] text" format.
func parseChecklistItemText(text string) ChecklistItem {
	if len(text) > 7 && text[:6] == "[open]" {
		return ChecklistItem{Text: text[7:], Status: "open"}
	}
	if len(text) > 7 && text[:6] == "[done]" {
		return ChecklistItem{Text: text[7:], Status: "done"}
	}
	return ChecklistItem{Text: text, Status: "open"}
}

// buildChecklistADF constructs an ADF document for the checklist.
func buildChecklistADF(items []ChecklistItem) map[string]interface{} {
	listItems := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		prefix := fmt.Sprintf("[%s] ", item.Status)
		listItems = append(listItems, map[string]interface{}{
			"type": "listItem",
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": prefix + item.Text,
						},
					},
				},
			},
		})
	}

	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": []map[string]interface{}{
			{
				"type": "orderedList",
				"attrs": map[string]interface{}{
					"order": 1,
				},
				"content": []map[string]interface{}{
					{
						"type": "listItem",
						"content": []map[string]interface{}{
							{
								"type": "paragraph",
								"content": []map[string]interface{}{
									{
										"type": "text",
										"text": "Default checklist",
									},
								},
							},
						},
					},
				},
			},
			{
				"type":    "bulletList",
				"content": listItems,
			},
		},
	}
}
