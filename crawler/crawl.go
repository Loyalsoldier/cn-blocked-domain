package crawler

import (
	"compress/gzip"
	"net/http"
	"runtime"
)

func genUA() (userAgent string) {
	switch runtime.GOOS {
	case "linux":
		userAgent = `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:76.0) Gecko/20100101 Firefox/76.0`
	case "darwin":
		userAgent = `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.129 Safari/537.36`
	case "windows":
		userAgent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:76.0) Gecko/20100101 Firefox/76.0`
	}
	return
}

// Crawl crawls webpage content and returns *gzip.Reader
func Crawl(url, ref string) (*gzip.Reader, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var ua = genUA()
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", ref)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	ungzipData, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, err
	}
	return ungzipData, nil
}
