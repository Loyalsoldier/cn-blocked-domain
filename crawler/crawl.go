package crawler

import (
	"compress/gzip"
	"net/http"
)

func Crawl(url, ref string) (*gzip.Reader, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.129 Safari/537.36")
	req.Header.Set("Referer", ref)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	ungzipData, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, err
	}
	return ungzipData, err
}
