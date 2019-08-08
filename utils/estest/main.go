package main

import (
	"fmt"

	"github.com/notque/elktools/pkg/elasticsearch"
)

func main() {
	esclient := elasticsearch.Connect()
	fmt.Printf("%+v\n", esclient)
}
