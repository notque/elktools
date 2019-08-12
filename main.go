package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tidwall/sjson"

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
		//eventtype := line.Get("type").String()
		//fmt.Printf("EventTime: %s\n", eventtime)
		//fmt.Printf("ProjectID: %s\n", projectid)
		//fmt.Printf("Type: %s\n", eventtype)
		// Change type from clone_for_swift to clone_for_audit
		// which will catch duplicate events being loaded.
		data := line.String()
		eventline, err := sjson.Set(data, "type", "clone_for_audit")
		//fmt.Printf("EventLine: %s\n", eventline)
		if err != nil {
			log.Fatalf("Failed to edit event: %s\n", err)
		}
		index := elasticsearch.CreateIndexName(projectid, eventtime)
		fmt.Printf("IndexName: %s\n", index)
		// Call elastisearch loading with a functional bit.
		//if 1 == 0 {
		elasticsearch.LoadEvent(eventline, index, es)
		//}
		return true
	})
}

type Events struct {
	event []cadf.Event
}
