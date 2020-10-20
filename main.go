package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/Loyalsoldier/cn-blocked-domain/utils"
)

const (
	initElem     = "ul.pager"
	initHrefElem = ".pager-last.last a"
	elem         = "table.gf-header tbody tr"
	uElem        = "td.first a"
	bElem        = "td.blocked"
	re           = `^\/(https?\/)?([a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)$`
	reForIP      = `(([0-9]{1,3}\.){3}[0-9]{1,3})`
	rawFile      = "raw.txt"
	filteredFile = "domains.txt"
	percentStd   = 50       // set the min percent to filter domains
	retryTimes   = 3        // set crawler max retry times
	maxCap       = 100 * 16 // set the capacity of channel to contain results
)

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
	resultSlice := make(sortableSlice, 0, len(r))
	reg := regexp.MustCompile(reForIP)
	for domainKey := range r {
		if len(reg.FindStringSubmatch(domainKey)) > 0 {
			continue
		}
		resultSlice = append(resultSlice, domainKey)
	}

	sort.Stable(resultSlice)
	return buildTreeAndUnique(resultSlice)
}

func main() {
	orginalCPUs, numCPUs := utils.SetGOMAXPROCS()

	fmt.Println("CPU cores: ", utils.Info(orginalCPUs))
	fmt.Println("Go Processors: ", utils.Info(numCPUs))

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

	resultChan := make(chan map[string]int, maxCap)

	go ControlFlow(crawlItems, resultChan, elem, uElem, bElem, retryTimes, numCPUs)
	ValidateAndWrite(resultChan, filteredFile, rawFile, re, reForIP, percentStd)
}
