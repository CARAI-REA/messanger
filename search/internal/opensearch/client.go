package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

type Client struct {
	client *opensearch.Client
	index  string
}

type MessageDoc struct {
	MessageID int64  `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	SenderID  int64  `json:"sender_id"`
	Text      string `json:"text"`
}

type Hit struct {
	MessageID int64
	ChatID    int64
	SenderID  int64
	Text      string
	Score     float64
}

func New(url, index string) (*Client, error) {
	c, err := opensearch.NewClient(opensearch.Config{Addresses: []string{url}})
	if err != nil {
		return nil, err
	}
	cli := &Client{client: c, index: index}
	_ = cli.EnsureIndex(context.Background())
	return cli, nil
}

func (c *Client) EnsureIndex(ctx context.Context) error {
	res, err := c.client.Indices.Exists([]string{c.index})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		return nil
	}
	body := `{
	  "settings": {"number_of_shards": 1, "number_of_replicas": 0},
	  "mappings": {
	    "properties": {
	      "message_id": {"type": "long"},
	      "chat_id": {"type": "long"},
	      "sender_id": {"type": "long"},
	      "text": {"type": "text"}
	    }
	  }
	}`
	req := opensearchapi.IndicesCreateRequest{Index: c.index, Body: strings.NewReader(body)}
	cres, err := req.Do(ctx, c.client)
	if err != nil {
		return err
	}
	defer cres.Body.Close()
	if cres.IsError() {
		b, _ := io.ReadAll(cres.Body)
		return fmt.Errorf("create index: %s", string(b))
	}
	return nil
}

func (c *Client) IndexMessage(ctx context.Context, doc MessageDoc) error {
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	id := strconv.FormatInt(doc.MessageID, 10)
	req := opensearchapi.IndexRequest{
		Index:      c.index,
		DocumentID: id,
		Body:       bytes.NewReader(b),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		raw, _ := io.ReadAll(res.Body)
		return fmt.Errorf("index: %s", string(raw))
	}
	return nil
}

func (c *Client) DeleteMessage(ctx context.Context, messageID int64) error {
	req := opensearchapi.DeleteRequest{Index: c.index, DocumentID: strconv.FormatInt(messageID, 10)}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (c *Client) UpdateMessageText(ctx context.Context, messageID int64, text string) error {
	body := fmt.Sprintf(`{"doc":{"text":%q}}`, text)
	req := opensearchapi.UpdateRequest{
		Index:      c.index,
		DocumentID: strconv.FormatInt(messageID, 10),
		Body:       strings.NewReader(body),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (c *Client) SearchMessages(ctx context.Context, query string, chatID int64, from, size int) ([]Hit, error) {
	must := []map[string]any{
		{"match": map[string]any{"text": query}},
	}
	if chatID > 0 {
		must = append(must, map[string]any{"term": map[string]any{"chat_id": chatID}})
	}
	bodyMap := map[string]any{
		"from": from,
		"size": size,
		"query": map[string]any{
			"bool": map[string]any{"must": must},
		},
	}
	b, _ := json.Marshal(bodyMap)
	res, err := c.client.Search(
		c.client.Search.WithContext(ctx),
		c.client.Search.WithIndex(c.index),
		c.client.Search.WithBody(bytes.NewReader(b)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		raw, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search: %s", string(raw))
	}
	var parsed struct {
		Hits struct {
			Hits []struct {
				Score  float64    `json:"_score"`
				Source MessageDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]Hit, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		out = append(out, Hit{
			MessageID: h.Source.MessageID,
			ChatID:    h.Source.ChatID,
			SenderID:  h.Source.SenderID,
			Text:      h.Source.Text,
			Score:     h.Score,
		})
	}
	return out, nil
}
