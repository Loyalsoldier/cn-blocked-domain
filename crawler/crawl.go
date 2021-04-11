package crawler

import (
	"errors"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
)

func genUA() (userAgent string) {
	switch runtime.GOOS {
	case "linux":
		userAgent = `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0`
	case "darwin":
		userAgent = `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.129 Safari/537.36`
	case "windows":
		userAgent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:87.0) Gecko/20100101 Firefox/87.0`
	}
	return
}

// Crawl crawls webpage content and returns *gzip.Reader
func Crawl(target, referer string) (*http.Response, error) {
	if _, err := url.Parse(target); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", genUA())
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")

	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("bad status code: " + strconv.Itoa(resp.StatusCode))
	}

	return resp, nil
}
