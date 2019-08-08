package main

import (
	"fmt"
	"log"
	"os"

	"github.com/olivere/elastic"

	"github.com/tidwall/gjson"

	"github.com/notque/elktools/pkg/elasticsearch"
	"github.com/notque/elktools/pkg/swift"
	"github.com/sapcc/hermes/pkg/cadf"
)

func main() {
	s, err := swift.NewSwift(os.Getenv("OS_CONTAINER"))
	if err != nil {
		log.Fatalf("Failed to initialize swift backend: %s", err)
	}

	var es *elastic.Client
	es = elasticsearch.Connect()
	if err != nil {
		log.Fatalf("Failed to created ElasticSearch connection: %s", err)
	}

	var events string
	events, err = swift.ContentsAsString(s)
	if err != nil {
		log.Fatalf("Failed to list contents of container: %s", err)
	}
	gjson.ForEachLine(events, func(line gjson.Result) bool {
		println(line.String())
		eventtime := line.Get("eventTime").String()
		projectid := line.Get("initiator.project_id").String()
		fmt.Printf("EventTime: %s\n", eventtime)
		fmt.Printf("ProjectID: %s\n", projectid)
		index := elasticsearch.CreateIndexName(projectid, eventtime)
		fmt.Printf("IndexName: %s\n", index)
		// Call elastisearch loading with a functional bit.
		if 1 == 0 {
			elasticsearch.LoadEvent(line.String(), index, es)
		}
		return true
	})
}

type Events struct {
	event []cadf.Event
}
