package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regexp"
	"sort"

	"github.com/olivere/elastic" // <- should end with /v6, but missing due to compatibility reasons
)

// sem is a channel that will allow up to 10 concurrent operations.
var sem = make(chan int, 20)

func main() {
	var (
		url   = flag.String("url", "http://localhost:9200", "Elasticsearch URL")
		sniff = flag.Bool("sniff", false, "Enable or disable sniffing")
	)
	flag.Parse()
	log.SetFlags(0)

	if *url == "" {
		*url = "http://127.0.0.1:9200"
	}

	// Create an Elasticsearch client
	client, err := elastic.NewClient(elastic.SetURL(*url), elastic.SetSniff(*sniff))
	if err != nil {
		log.Fatal(err)
	}

	// Just a status message
	fmt.Println("Connection succeeded")

	indexes, err := client.IndexNames()
	if err != nil {
		log.Fatal(err)
	}

	sort.Strings(indexes)
	//fmt.Println(strings.Join(indexes, "\n"))
	fmt.Println("Number of indexes: ", len(indexes))

	for _, index := range indexes {
		sem <- 1
		go func(index string) {
			dailytomonthly(client, index)
			<-sem
		}(index)
	}

	//Wait for all goroutines to finish
	for i := 0; i < cap(sem); i++ {
		sem <- 1
	}

	//settings := client.IndexGet(indexes[len(indexes)-1])
	//fmt.Printf("settings %v", settings)

	version, err := client.ElasticsearchVersion(*url)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Elasticsearch version %s\n", version)
}

func dailytomonthly(client *elastic.Client, index string) {
	if validindex(index) {
		monindex := monthlyindex(index)
		//fmt.Println(monindex)
		exists := checkindex(client, monindex)
		if !exists {
			createindex(client, monindex)
		}
		reindex(client, index, monindex)
		closeindex(client, index)
	}
}

func reindex(client *elastic.Client, srcindex string, dstindex string) {
	src := elastic.NewReindexSource().Index(srcindex)
	dst := elastic.NewReindexDestination().Index(dstindex)
	res, err := client.Reindex().Source(src).Destination(dst).Refresh("true").Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("src: %s, dest: %s :Reindexed a total of %d documents\n", srcindex, dstindex, res.Total)
}

//Testing for now to validate things work by using a subset of indexes
func validindex(index string) bool {
	var validIndex = regexp.MustCompile(`^audit.*201\d\.\d\d\.\d\d`)
	//fmt.Printf("Index: %s is %t\n", index, validIndex.MatchString(index))
	if !validIndex.MatchString(index) {
		return false
	}
	return true
}

//convert daily index to monthly
func monthlyindex(index string) (monthlyindex string) {
	re, err := regexp.Compile(`\.\d\d$`)
	if err != nil {
		fmt.Errorf("Error matching regex for monthly index: %s", err)
		return ""
	}
	monthlyindex = re.ReplaceAllString(index, ``)
	return monthlyindex
}

//Does index exist?
func checkindex(client *elastic.Client, index string) bool {
	ctx := context.Background()
	exists, err := client.IndexExists(index).Do(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if !exists {
		return false
	}
	return true
}

func createindex(client *elastic.Client, index string) {
	ctx := context.Background()
	createIndex, err := client.CreateIndex(index).Do(ctx)
	if err != nil {
		fmt.Printf("Error creating index: %s", err)
		return
	}
	if !createIndex.Acknowledged {
		// Not acknowledged
		fmt.Printf("Index was not acknowledged as created: %s\n", index)
	}
	fmt.Printf("Index created: %s\n", index)
}

func closeindex(client *elastic.Client, index string) {
	ctx := context.Background()
	closeIndex, err := client.CloseIndex(index).Do(ctx)
	if err != nil {
		panic(err)
	}
	if !closeIndex.Acknowledged {
		//Not acknowledged
		fmt.Printf("Index was not acknowledged as deleted: %s\n", index)
	}
	fmt.Printf("Index closed: %s\n", index)
}
func deleteindex(client *elastic.Client, index string) {
	ctx := context.Background()
	deleteIndex, err := client.DeleteIndex(index).Do(ctx)
	if err != nil {
		panic(err)
	}
	if !deleteIndex.Acknowledged {
		//Not acknowledged
		fmt.Printf("Index was not acknowledged as deleted: %s\n", index)
	}
	fmt.Printf("Index deleted: %s\n", index)
}

//Count offline values afterwards instead of doing it in the migration.
// Reindex docs based on field value.... Pod name starting with "swift" for example.
// See if you can't do this concurrent.
