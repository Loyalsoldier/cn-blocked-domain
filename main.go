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
	"github.com/Loyalsoldier/cn-blocked-domain/utils"
	"github.com/PuerkitoBio/goquery"
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

// CrawlType defines the structure of AlexaTop1000 type of URLs and list
type CrawlType struct {
	GreatFireURL *GreatFireURL
	URLList      []string
	mux          sync.RWMutex
}

// NewURLList returns the URL list to be crawled
func (c *CrawlType) NewURLList() {
	c.mux.Lock()
	c.URLList = make([]string, 0)
	for i := 0; i < c.GreatFireURL.MaxPage; i++ {
		fullURL := c.GreatFireURL.BaseURL + c.GreatFireURL.MiddleURL + c.GreatFireURL.SuffixURL + strconv.Itoa(i)
		c.URLList = append(c.URLList, fullURL)
	}
	defer c.mux.Unlock()
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

// GetMaxPage gets the max page of crawl type
func GetMaxPage(initURLSlice map[*CrawlType]string, initElem, initHrefElem string) {
	for crawlType, initURL := range initURLSlice {
		ungzipData, err := crawler.Crawl(initURL, "https://zh.greatfire.org")
		if err != nil {
			log.Fatal(err)
		}
		defer ungzipData.Close()

		// Load the HTML document
		doc, err := goquery.NewDocumentFromReader(ungzipData)

		// Find items
		doc.Find(initElem).Each(func(i int, s *goquery.Selection) {
			// For each item found, get contents
			lastPageHref, exists := s.Find(initHrefElem).Attr("href")
			if exists {
				matchList := strings.Split(lastPageHref, "?page=")
				if len(matchList) > 0 {
					maxPage := matchList[1]
					crawlType.GreatFireURL.MaxPage, _ = strconv.Atoi(maxPage)
					log.Printf("%s has %s pages\n", initURL, maxPage)
				}
			} else {
				log.Printf("Failed to get the max page of %s\n", initURL)
			}
		})
	}
}

// ControlFlow controls the crawl process
func ControlFlow(crawlItems []string, outChan chan map[string]int, elem, uElem, bElem, rElem string, lenItems, retryTimes, numCPUs int) {
	maxGoRoutinesChan := make(chan int, numCPUs)
	doneChan := make(chan Done, lenItems)

	for _, url := range crawlItems {
		maxGoRoutinesChan <- 1
		go CrawlAndProcessPage(url, outChan, doneChan, maxGoRoutinesChan, elem, uElem, bElem, rElem, retryTimes)
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

		if attempt > 1 {
			log.Println(utils.Fatal(attempt), "time, crawling URL:", utils.Info(url))
		}
		log.Println(utils.Warning(attempt), "time, crawling URL:", utils.Info(url))

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
					domain = matchList[len(matchList)-2]
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
		initElem     = "ul.pager"
		initHrefElem = ".pager-last.last a"
		elem         = "table.gf-header tbody tr"
		uElem        = "td.first a"
		bElem        = "td.blocked"
		rElem        = "td.restricted"
		re           = `^\/(https?\/)?([a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)$`
		reForIP      = `(([0-9]{1,3}\.){3}[0-9]{1,3})`
		rawFile      = "raw.txt"
		filteredFile = "temp-domains.txt"
		percentStd   = 50 // set the min percent to filter domains
		retryTimes   = 3  // set crawler max retry times
	)

	// Set Go processors no less than 16
	numCPUs := runtime.NumCPU()
	if numCPUs < 8 {
		numCPUs = 8
	}
	runtime.GOMAXPROCS(numCPUs)

	alexaTop1000 := &CrawlType{
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "alexa-top-1000-domains",
			SuffixURL: "?page="}}

	blocked := &CrawlType{
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "blocked",
			SuffixURL: "?page="}}

	domains := &CrawlType{
		GreatFireURL: &GreatFireURL{
			BaseURL:   "https://zh.greatfire.org/search/",
			MiddleURL: "domains",
			SuffixURL: "?page="}}

	initURLSlice := make(map[*CrawlType]string)
	initURLSlice[alexaTop1000] = "https://zh.greatfire.org/search/alexa-top-1000-domains?page=0"
	initURLSlice[blocked] = "https://zh.greatfire.org/search/blocked?page=0"
	initURLSlice[domains] = "https://zh.greatfire.org/search/domains?page=0"

	// Get CrawlType max page
	GetMaxPage(initURLSlice, initElem, initHrefElem)

	// Generates each type's URLList
	alexaTop1000.NewURLList()
	blocked.NewURLList()
	domains.NewURLList()

	// Generate items to be crawled
	crawlItems := make([]string, 0)
	for crawlType := range initURLSlice {
		for _, url := range crawlType.URLList {
			crawlItems = append(crawlItems, url)
		}
	}

	lenItems := len(crawlItems)
	resultChan := make(chan map[string]int, LIMIT)

	go ControlFlow(crawlItems, resultChan, elem, uElem, bElem, rElem, lenItems, retryTimes, numCPUs)
	ValidateAndWrite(resultChan, filteredFile, rawFile, re, reForIP, percentStd)
}
