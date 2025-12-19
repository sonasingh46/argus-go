package es

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/elastic/go-elasticsearch/v8"
)

type Client struct {
	ES *elasticsearch.Client
}

func New(addresses []string) *Client {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: addresses,
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to Elasticsearch: %s", err)
	}
	return &Client{ES: client}
}

func (c *Client) FetchThresholdRules() ([]map[string]interface{}, error) {
	res, err := c.ES.Search(c.ES.Search.WithIndex("alert_rules"))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var r map[string]interface{}
	json.NewDecoder(res.Body).Decode(&r)

	var rules []map[string]interface{}
	hitsObj, ok := r["hits"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	for _, hit := range hitsObj["hits"].([]interface{}) {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		rules = append(rules, source)
	}
	return rules, nil
}

func (c *Client) Search(index string, query map[string]interface{}) (map[string]interface{}, error) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(query)
	res, err := c.ES.Search(c.ES.Search.WithIndex(index), c.ES.Search.WithBody(&buf))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var r map[string]interface{}
	json.NewDecoder(res.Body).Decode(&r)
	return r, nil
}
