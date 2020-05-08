package parser

import (
	"compress/gzip"
	"log"
	"strconv"

	"github.com/Loyalsoldier/cn-blocked-domain/errorer"
	"github.com/PuerkitoBio/goquery"
)

// HTMLParser parses webpage content and sends URL & percent map to channel
func HTMLParser(resultChan chan map[string]int, data *gzip.Reader, elem, uElem, bElem, rElem string) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Runtime panic: %v\n", err)
		}
	}()

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(data)
	errorer.CheckError(err)

	// Find items
	doc.Find(elem).Each(func(i int, s *goquery.Selection) {
		// For each item found, get contents
		url, _ := s.Find(uElem).Attr("href")
		bPerStr := s.Find(bElem).Text()
		rPerStr := s.Find(rElem).Text()

		var blockPerNum, restrictPerNum, percent int
		if bPerStr != "" {
			blockPerNum, _ = strconv.Atoi(bPerStr[:len(bPerStr)-1])
			percent = blockPerNum
		}
		if rPerStr != "" {
			restrictPerNum, _ = strconv.Atoi(rPerStr[:len(rPerStr)-1])
			percent = restrictPerNum
		}

		result := make(map[string]int)
		result[url] = percent
		resultChan <- result
	})
}
