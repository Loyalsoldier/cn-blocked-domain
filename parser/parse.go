package parser

import (
	"compress/gzip"
	"fmt"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

func HtmlParser(resultChan chan map[string]int, data *gzip.Reader, elem, uElem, bElem, rElem string) {
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(data)
	if err != nil {
		fmt.Println(err)
	}

	// Find items
	doc.Find(elem).Each(func(i int, s *goquery.Selection) {
		// For each item found, get contents
		domain := s.Find(uElem).Text()
		bPerStr := s.Find(bElem).Text()
		rPerStr := s.Find(rElem).Text()

		var blockPerNum, restrictPerNum, percent int
		result := make(map[string]int)
		if bPerStr != "" {
			blockPerNum, _ = strconv.Atoi(bPerStr[:len(bPerStr)-1])
			percent = blockPerNum
		}
		if rPerStr != "" {
			restrictPerNum, _ = strconv.Atoi(rPerStr[:len(rPerStr)-1])
			percent = restrictPerNum
		}

		result[domain] = percent
		resultChan <- result
	})
}
