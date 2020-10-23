package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Loyalsoldier/cn-blocked-domain/crawler"
	"github.com/Loyalsoldier/cn-blocked-domain/parser"
	"github.com/Loyalsoldier/cn-blocked-domain/utils"
	"github.com/PuerkitoBio/goquery"
	"github.com/matryer/try"
)

type sortableSlice []string

func (r sortableSlice) Len() int {
	return len(r)
}

func (r sortableSlice) Less(i, j int) bool {
	return len(strings.Split(r[i], ".")) < len(strings.Split(r[j], "."))
}

func (r sortableSlice) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
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
func ControlFlow(crawlItems []string, outChan chan map[string]int, elem, uElem, bElem string, retryTimes, numCPUs int) {
	var wg sync.WaitGroup
	maxGoRoutinesChan := make(chan struct{}, numCPUs)

	for _, url := range crawlItems {
		// Decrement the remaining space for max GoRoutines parallelism
		maxGoRoutinesChan <- struct{}{}
		// Increment the WaitGroup counter
		wg.Add(1)
		go CrawlAndProcessPage(url, outChan, &wg, maxGoRoutinesChan, elem, uElem, bElem, retryTimes)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	close(outChan)
}

// CrawlAndProcessPage crawls a URL page and processes it
func CrawlAndProcessPage(url string, outChan chan map[string]int, wg *sync.WaitGroup, maxGoRoutinesChan chan struct{}, elem, uElem, bElem string, retryTimes int) {
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
		} else {
			log.Println(utils.Warning(attempt), "time, crawling URL:", utils.Info(url))
		}

		ungzipData, err = crawler.Crawl(url, "https://zh.greatfire.org")
		utils.CheckError(err)
		return
	})
	utils.CheckError(err)
	defer ungzipData.Close()

	parser.HTMLParser(outChan, ungzipData, elem, uElem, bElem)

	// Decrement the counter when the goroutine completes
	defer wg.Done()
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
	utils.CheckError(err)
	defer f.Close()

	g, err := os.OpenFile(filteredFile, os.O_WRONLY|os.O_CREATE, 0644)
	utils.CheckError(err)
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
	sort.Slice(resultSlice, func(i, j int) bool {
		return resultSlice[i] < resultSlice[j]
	})

	for _, domain := range resultSlice {
		// Write filtered result to temp-domains.txt file
		x := bufio.NewWriter(g)
		x.WriteString(domain + "\n")
		x.Flush()
	}
}

func buildTreeAndUnique(sortedDomainList []string) []string {
	tree := newList()
	remainList := make([]string, 0, len(sortedDomainList))

	for _, domain := range sortedDomainList {
		parts := strings.Split(domain, ".")
		leafIdx, isInserted, err := tree.Insert(parts)

		if err != nil {
			log.Println(utils.Fatal("[Error]"), "check domain", utils.Info(domain), "for redundancy.")
			continue
		}
		if !isInserted {
			redundantParts := make([]string, 0, len(parts))
			for i := 0; i <= leafIdx; i++ {
				redundantParts = append(redundantParts, parts[i])
			}
			redundantStr := strings.Join(redundantParts, ".")
			log.Println("Found redundant domain:", utils.Info(domain), "@", utils.Warning(redundantStr))
			continue
		}
		remainList = append(remainList, domain)
	}

	return remainList
}
