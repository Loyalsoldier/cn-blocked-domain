package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	crawler "github.com/Loyalsoldier/cn-blocked-domain/crawler"
	parser "github.com/Loyalsoldier/cn-blocked-domain/parser"
)

// LIMIT sets the capacity of channel to contain results
const LIMIT = 100 * 16

// Done implies whether the URL has been crawled or not
type Done bool

// GreatFireURL defines the structure of the format of URL
type GreatFireURL struct {
	BaseURL   string
	MiddleURL string
	SuffixURL string
	MaxPage   int
}

// AlexaTop1000Type defines the structure of AlexaTop1000 type of URLs and list
type AlexaTop1000Type struct {
	GreatFireURL *GreatFireURL
	URLList      map[string]Done
	mux          sync.RWMutex
}

// NewURLList returns the URLs of type AlexaTop1000 to be crawled
func (a *AlexaTop1000Type) NewURLList() {
	a.mux.Lock()
	a.URLList = make(map[string]Done)
	for i := 0; i < a.GreatFireURL.MaxPage; i++ {
		fullURL := a.GreatFireURL.BaseURL + a.GreatFireURL.MiddleURL + a.GreatFireURL.SuffixURL + strconv.Itoa(i)
		a.URLList[fullURL] = false
	}
	a.mux.Unlock()
}

// BlockedType defines the structure of Blocked type of URLs and list
type BlockedType struct {
	GreatFireURL *GreatFireURL
	URLList      map[string]Done
	mux          sync.RWMutex
}

// NewURLList returns the URLs of type Blocked to be crawled
func (b *BlockedType) NewURLList() {
	b.mux.Lock()
	b.URLList = make(map[string]Done)
	for i := 0; i < b.GreatFireURL.MaxPage; i++ {
		fullURL := b.GreatFireURL.BaseURL + b.GreatFireURL.MiddleURL + b.GreatFireURL.SuffixURL + strconv.Itoa(i)
		b.URLList[fullURL] = false
	}
	b.mux.Unlock()
}

// DomainsType defines the structure of Domains type of URLs and list
type DomainsType struct {
	GreatFireURL *GreatFireURL
	URLList      map[string]Done
	StopPage     int
	mux          sync.RWMutex
}

// NewURLList returns the URLs of type Domains to be crawled
func (d *DomainsType) NewURLList() {
	d.mux.Lock()
	d.URLList = make(map[string]Done)
	for i := 0; i < d.GreatFireURL.MaxPage; i++ {
		fullURL := d.GreatFireURL.BaseURL + d.GreatFireURL.MiddleURL + d.GreatFireURL.SuffixURL + strconv.Itoa(i)
		d.URLList[fullURL] = false
	}
	d.mux.Unlock()
}

// Results defines the structure of domain result
type Results struct {
	domains []map[string]int
	mux     sync.Mutex
}

// Get crawles the URLs
func Get(inChan chan map[string]Done, outChan chan map[string]int, elem, uElem, bElem, rElem, re string, lenItems int) {
	doneChan := make(chan Done, lenItems)
	for urlMap := range inChan {
		go func(urlMap map[string]Done, doneChan chan Done, outChan chan map[string]int) {
			for url := range urlMap {
				ungzipData, err := crawler.Crawl(url, "https://zh.greatfire.org")
				if err != nil {
					// return
					fmt.Println(err)
				}
				defer ungzipData.Close()

				parser.HtmlParser(outChan, ungzipData, elem, uElem, bElem, rElem, re)
			}
			doneChan <- true
		}(urlMap, doneChan, outChan)
	}

	for i := 0; i < lenItems; i++ {
		<-doneChan
	}
	close(doneChan)
	close(outChan)
}

func main() {
	const (
		elem     = "table.gf-header tbody tr"
		uElem    = "td.first a"
		bElem    = "td.blocked"
		rElem    = "td.restricted"
		re       = `([a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)`
		filename = "blockedDomains.txt"
	)

	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)

	alexaTop1000 := &AlexaTop1000Type{
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "alexa-top-1000-domains",
			SuffixURL: "?page=",
			MaxPage:   7}}

	blocked := &BlockedType{
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "blocked",
			SuffixURL: "?page=",
			MaxPage:   13}}

	domains := &DomainsType{
		StopPage: 123,
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "domains",
			SuffixURL: "?page=",
			MaxPage:   1292}}

	alexaTop1000.NewURLList()
	blocked.NewURLList()
	domains.NewURLList()

	crawlItems := make([]map[string]Done, 0)
	for url, isDone := range alexaTop1000.URLList {
		item := map[string]Done{url: isDone}
		crawlItems = append(crawlItems, item)
	}
	for url, isDone := range blocked.URLList {
		item := map[string]Done{url: isDone}
		crawlItems = append(crawlItems, item)
	}
	// for url, isDone := range domains.URLList {
	// 	item := map[string]Done{url: isDone}
	// 	crawlItems = append(crawlItems, item)
	// }
	lenItems := len(crawlItems)

	inputChan := make(chan map[string]Done, numCPUs)
	resultChan := make(chan map[string]int, LIMIT)

	go func(crawlItems []map[string]Done, inputChan chan map[string]Done) {
		for _, item := range crawlItems {
			inputChan <- item
		}
		close(inputChan)
	}(crawlItems, inputChan)

	go Get(inputChan, resultChan, elem, uElem, bElem, rElem, re, lenItems)

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	for result := range resultChan {
		for url, percent := range result {
			w := bufio.NewWriter(f)
			w.WriteString(fmt.Sprintf("%s | %d\n", url, percent))
			w.Flush()
			fmt.Printf("%s | %d\n", url, percent)
		}
	}
}
