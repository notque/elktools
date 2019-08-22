package elasticsearch

import (
	"context"
	"fmt"
	"time"

	"github.com/olivere/elastic"
)

// Connect to Elasticsearch
func Connect(elasticHost string) *elastic.Client {
	// Create a client
	client, err := elastic.NewClient(elastic.SetURL(elasticHost), elastic.SetSniff(false))
	if err != nil {
		// Handle error
		panic(err)
	}

	return client
}

// CreateIndexIfNotExist Indexes are not created automatically, so we must create.
func CreateIndexIfNotExist(indexName string, es *elastic.Client) error {
	//
	ctx := context.Background()
	exists, err := es.IndexExists(indexName).Do(ctx)
	if err != nil {
		return err
	}

	if !exists {
		// Create a new index.
		createIndex, err := es.CreateIndex(indexName).BodyString(Mapping()).Do(ctx)
		if err != nil {
			// Handle error
			panic(err)
		}
		if !createIndex.Acknowledged {
			// Not acknowledged
		}
	}

	// Create Index

	return nil
}

//LoadEvent loads jsonline data into ElasticSearch
func LoadEvent(data string, index string, eventType string, es *elastic.Client) {

	err := CreateIndexIfNotExist(index, es)
	if err != nil {
		fmt.Printf("Could not Create Index: %s", err)
	}

	var put1 *elastic.IndexResponse
	put1, err = es.Index().
		Index(index).
		Type(eventType).
		BodyString(data).
		Do(context.TODO())

	if err != nil {
		// Handle error
		fmt.Printf("Could not load data into ES: %s", err)
	}
	if put1 != nil {
		fmt.Printf("Indexed Event %s to index %s, type %s\n", put1.Id, put1.Index, put1.Type)
	}
	return
}

// CreateIndexName Convienance function to get correct index for ElasticSearch
// EventTime?
func CreateIndexName(tenantID string, eventtime string) string {
	// Default Index is audit-default-%{+YYYY.MM}
	ym := time.Now().Format("2006.01")
	//fmt.Printf("Time: %s", ym)
	index := "audit-default-" + ym

	if tenantID != "" {
		//index = fmt.Sprintf("audit-%s-*", tenantID)
		index = "audit-" + tenantID + "-6-" + ym
	}
	//fmt.Printf("Index: %s", index)
	return index
}

//Mapping returns the audit mapping for Hermes.
func Mapping() string {
	mapping := `
{
	"mappings": {
	  "doc": {
		"properties": {
		  "@timestamp": {
			  "type": "date"
		  },
		  "@version": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "_unique_id": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "action": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "attachments": {
			  "properties": {
			  "content": {
				"type": "text",
				"fields": {
				  "raw": {
					"type": "keyword",
					"ignore_above": 256
				  }
				}
			  },
			  "name": {
				"type": "text",
				"fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				}
			  },
			  "typeURI": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  }
			  }
		  },
		  "eventTime": {
			  "type": "date"
		  },
		  "eventType": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "id": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "initiator": {
			  "properties": {
			  "domain": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "domain_id": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "host": {
				  "properties": {
				  "address": {
					  "type": "text",
					  "fields": {
					  "raw": {
						  "type": "keyword",
						  "ignore_above": 256
					  }
					  }
				  },
				  "agent": {
					  "type": "text",
					  "fields": {
					  "raw": {
						  "type": "keyword",
						  "ignore_above": 256
					  }
					  }
				  }
				  }
			  },
			  "id": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "name": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "project_id": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "typeURI": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  }
			  }
		  },
		  "observer": {
			  "properties": {
			  "id": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "name": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "typeURI": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  }
			  }
		  },
		  "outcome": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "reason": {
			  "properties": {
			  "reasonCode": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "reasonType": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  }
			  }
		  },
		  "requestPath": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "target": {
			  "properties": {
			  "attachments": {
				  "properties": {
				  "content": {
					  "type": "text",
					  "fields": {
					  "raw": {
						  "type": "keyword",
						  "doc_values": false,
						  "ignore_above": 256
					  }
					  }
				  },
				  "name": {
					  "type": "text",
					  "fields": {
					  "raw": {
						  "type": "keyword",
						  "ignore_above": 256
					  }
					  }
				  },
				  "typeURI": {
					  "type": "text",
					  "fields": {
					  "raw": {
						  "type": "keyword",
						  "ignore_above": 256
					  }
					  }
				  }
				  }
			  },
			  "domain_id": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "id": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "name": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "project_id": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  },
			  "typeURI": {
				  "type": "text",
				  "fields": {
				  "raw": {
					  "type": "keyword",
					  "ignore_above": 256
				  }
				  }
			  }
			  }
		  },
		  "type": {
			  "type": "text",
			  "fields": {
			  "keyword": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  },
		  "typeURI": {
			  "type": "text",
			  "fields": {
			  "raw": {
				  "type": "keyword",
				  "ignore_above": 256
			  }
			  }
		  }
		}
	  }
	}
  }
`
	return mapping
}
