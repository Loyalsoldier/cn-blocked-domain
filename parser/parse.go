package parser

import (
	"compress/gzip"
	"log"
	"strconv"

	"github.com/Loyalsoldier/cn-blocked-domain/utils"
	"github.com/PuerkitoBio/goquery"
)

// HTMLParser parses webpage content and sends URL & percent map to channel
func HTMLParser(resultChan chan map[string]int, data *gzip.Reader, elem, uElem, bElem string) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Runtime panic: %v\n", err)
		}
	}()

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(data)
	utils.CheckError(err)

	// Find items
	doc.Find(elem).Each(func(i int, s *goquery.Selection) {
		// For each item found, get contents
		url, _ := s.Find(uElem).Attr("href")
		bPerStr := s.Find(bElem).Text()

		var blockPerNum, percent int
		if bPerStr != "" {
			blockPerNum, _ = strconv.Atoi(bPerStr[:len(bPerStr)-1])
			percent = blockPerNum
		}

		result := make(map[string]int)
		result[url] = percent
		resultChan <- result
	})
}
