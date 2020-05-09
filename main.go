package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Loyalsoldier/cn-blocked-domain/crawler"
	"github.com/Loyalsoldier/cn-blocked-domain/errorer"
	"github.com/Loyalsoldier/cn-blocked-domain/parser"
	"github.com/matryer/try"
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
	defer a.mux.Unlock()
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
	defer b.mux.Unlock()
}

// DomainsType defines the structure of Domains type of URLs and list
type DomainsType struct {
	GreatFireURL *GreatFireURL
	URLList      map[string]Done
	StopAtPage   int
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
	defer d.mux.Unlock()
}

// Results defines the structure of domain result map
type Results map[string]struct{}

// SortAndUnique filters the Results slice
func (r Results) SortAndUnique(reForIP string) []string {
	resultSlice := make([]string, 0, len(r))
	reg := regexp.MustCompile(reForIP)
	for domainKey := range r {
		matchList := reg.FindStringSubmatch(domainKey)
		if len(matchList) > 0 {
			continue
		}
		resultSlice = append(resultSlice, domainKey)
	}
	sort.Strings(resultSlice)
	return resultSlice
}

// ControlFlow controls the crawl process
func ControlFlow(crawlItems []map[string]Done, outChan chan map[string]int, elem, uElem, bElem, rElem string, lenItems, retryTimes, numCPUs int) {
	maxGoRoutinesChan := make(chan int, numCPUs)
	doneChan := make(chan Done, lenItems)

	for _, urlMap := range crawlItems {
		maxGoRoutinesChan <- 1
		for url := range urlMap {
			go CrawlAndProcessPage(url, outChan, doneChan, maxGoRoutinesChan, elem, uElem, bElem, rElem, retryTimes)
		}
	}

	// Wait for all goroutines to be completed
	for i := 0; i < lenItems; i++ {
		<-doneChan
	}
	close(doneChan)
	close(outChan)
}

// CrawlAndProcessPage crawls a URL page and processes it
func CrawlAndProcessPage(url string, outChan chan map[string]int, doneChan chan Done, maxGoRoutinesChan chan int, elem, uElem, bElem, rElem string, retryTimes int) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Goroutine panic: fetching %v : %v\n", url, err)
		}
	}()

	var ungzipData *gzip.Reader
	err := try.Do(func(attempt int) (retry bool, err error) {
		retry = attempt < retryTimes
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		ungzipData, err = crawler.Crawl(url, "https://zh.greatfire.org")
		errorer.CheckError(err)
		return
	})
	errorer.CheckError(err)
	defer ungzipData.Close()

	parser.HTMLParser(outChan, ungzipData, elem, uElem, bElem, rElem)

	// Indicate that this goroutine has completed
	doneChan <- true
	// Indicate that there is one free space in goroutine list
	<-maxGoRoutinesChan
}

// ValidateAndWrite filters urlMap from resultChan and writes it to files
func ValidateAndWrite(resultChan chan map[string]int, filteredFile, rawFile, re, reForIP string, percentStd int) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Runtime panic: %v\n", err)
		}
	}()

	f, err := os.OpenFile(rawFile, os.O_WRONLY|os.O_CREATE, 0644)
	errorer.CheckError(err)
	defer f.Close()

	g, err := os.OpenFile(filteredFile, os.O_WRONLY|os.O_CREATE, 0644)
	errorer.CheckError(err)
	defer g.Close()

	var resultMap Results = make(map[string]struct{})
	for result := range resultChan {
		for url, percent := range result {
			url = strings.ToLower(url)
			// Write raw result to raw.txt file
			w := bufio.NewWriter(f)
			w.WriteString(fmt.Sprintf("%s | %d\n", url, percent))
			w.Flush()

			if percent >= percentStd {
				var domain string
				reg := regexp.MustCompile(re)
				matchList := reg.FindStringSubmatch(url)

				if len(matchList) > 0 {
					domain = matchList[0]
					// Write filtered result to console
					fmt.Printf("%s | %d\n", domain, percent)
					// Write filtered result to Results type variable resultMap
					resultMap[domain] = struct{}{}
				}
			}
		}
	}

	resultSlice := resultMap.SortAndUnique(reForIP)
	for _, domain := range resultSlice {
		// Write filtered result to temp-domains.txt file
		x := bufio.NewWriter(g)
		x.WriteString(domain + "\n")
		x.Flush()
	}
}

func main() {
	const (
		elem              = "table.gf-header tbody tr"
		uElem             = "td.first a"
		bElem             = "td.blocked"
		rElem             = "td.restricted"
		re                = `([a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)`
		reForIP           = `(([0-9]{1,3}\.){3}[0-9]{1,3})`
		rawFile           = "raw.txt"
		filteredFile      = "temp-domains.txt"
		alexaMaxPage      = 7
		blockedMaxPage    = 935
		domainsMaxPage    = 1293
		domainsStopAtPage = 123
		percentStd        = 50 // set the min percent to filter domains
		retryTimes        = 3  // set crawler max retry times
	)

	// Set Go processors no less than 8
	numCPUs := runtime.NumCPU()
	if numCPUs < 8 {
		numCPUs = 8
	}
	runtime.GOMAXPROCS(numCPUs)

	alexaTop1000 := &AlexaTop1000Type{
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "alexa-top-1000-domains",
			SuffixURL: "?page=",
			MaxPage:   alexaMaxPage}}

	blocked := &BlockedType{
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "blocked",
			SuffixURL: "?page=",
			MaxPage:   blockedMaxPage}}

	domains := &DomainsType{
		StopAtPage: domainsStopAtPage,
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "domains",
			SuffixURL: "?page=",
			MaxPage:   domainsMaxPage}}

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
	for url, isDone := range domains.URLList {
		item := map[string]Done{url: isDone}
		crawlItems = append(crawlItems, item)
	}

	lenItems := len(crawlItems)
	resultChan := make(chan map[string]int, LIMIT)

	go ControlFlow(crawlItems, resultChan, elem, uElem, bElem, rElem, lenItems, retryTimes, numCPUs)
	ValidateAndWrite(resultChan, filteredFile, rawFile, re, reForIP, percentStd)
}
