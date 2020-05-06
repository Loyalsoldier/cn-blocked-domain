package parser

import (
	"compress/gzip"
	"fmt"
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

func HtmlParser(resultChan chan map[string]int, data *gzip.Reader, elem, uElem, bElem, rElem, re string) {
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(data)
	if err != nil {
		fmt.Println(err)
	}

	// Find items
	doc.Find(elem).Each(func(i int, s *goquery.Selection) {
		// For each item found, get contents
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

		if percent >= 50 {
			var domain string
			url, _ := s.Find(uElem).Attr("href")
			reg := regexp.MustCompile(re)
			matchList := reg.FindStringSubmatch(url)

			if len(matchList) > 0 {
				result := make(map[string]int)
				domain = matchList[0]
				result[domain] = percent
				resultChan <- result
			}
		}
	})
}
