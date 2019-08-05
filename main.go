package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sapcc/hermes/pkg/cadf"
)

func main() {
	// fmt.Printf("projectname: %s\n", os.Getenv("OS_PROJECT_NAME"))
	// fmt.Printf("domainname: %s\n", os.Getenv("OS_PROJECT_DOMAIN_NAME"))
	// fmt.Printf("container: %s\n", os.Getenv("OS_CONTAINER"))
	// fmt.Printf("test\n")
	s, err := NewSwift(os.Getenv("OS_CONTAINER"))
	if err != nil {
		log.Fatalf("Failed to initialize swift backend: %s", err)
	}
	var events []cadf.Event
	events, err = GetSwiftEventsJSONLines(s)
	if err != nil {
		log.Fatalf("Failed to list contents of container: %s", err)
	}

	for _, e := range events {
		fmt.Printf("ProjectID: %s\n", e.Initiator.ProjectID)
		fmt.Printf("%+v\n", e)
	}

	//log.Printf("Using %s swift", s)
}

type Events struct {
	event []cadf.Event
}
