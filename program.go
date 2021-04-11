package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"gopkg.in/yaml.v2"

	"github.com/Loyalsoldier/cn-blocked-domain/crawler"
	"github.com/Loyalsoldier/cn-blocked-domain/utils"
)

var (
	ErrConfigFormatNotSupported = errors.New("config format not supported")
	ErrConfigIsEmpty            = errors.New("config is empty")
	ErrCrawlConfigIsEmpty       = errors.New("crawl config is empty")
	ErrFilterConfigIsEmpty      = errors.New("filter config is empty")
	ErrCustomizeConfigIsEmpty   = errors.New("Customize config is empty")
	ErrInvalidPageNumber        = errors.New("invalid page number")
)

type URL struct {
	BaseURL       string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	InitSuffixURL string `yaml:"init_suffix_url,omitempty" json:"init_suffix_url,omitempty"`
	SuffixURL     string `yaml:"suffix_url,omitempty" json:"suffix_url,omitempty"`
}

type Type struct {
	Name    string `yaml:"name,omitempty" json:"name,omitempty"`
	TypeURL string `yaml:"type_url,omitempty" json:"type_url,omitempty"`
	Referer string `yaml:"referer,omitempty" json:"referer,omitempty"`
	IsCrawl bool   `yaml:"is_crawl,omitempty" json:"is_crawl,omitempty"`
	From    int    `yaml:"from,omitempty" json:"from,omitempty"`
	To      int    `yaml:"to,omitempty" json:"to,omitempty"`
}

type Elem struct {
	Container string `yaml:"container,omitempty" json:"container,omitempty"`
	Content   string `yaml:"content,omitempty" json:"content,omitempty"`
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
	Attr      string `yaml:"attr,omitempty" json:"attr,omitempty"`
	Splitter  string `yaml:"splitter,omitempty" json:"splitter,omitempty"`
}

type Crawl struct {
	*URL
	Types        []*Type `yaml:"types,omitempty" json:"types,omitempty"`
	InitElement  *Elem   `yaml:"init_element,omitempty" json:"init_element,omitempty"`
	CrawlElement *Elem   `yaml:"crawl_element,omitempty" json:"crawl_element,omitempty"`
}

type FilterType struct {
	Domain string `yaml:"domain,omitempty" json:"domain,omitempty"`
	IP     string `yaml:"ip,omitempty" json:"ip,omitempty"`
}

type Filter struct {
	Regexp  *FilterType `yaml:"regexp,omitempty" json:"regexp,omitempty"`
	Percent int         `yaml:"percent,omitempty" json:"percent,omitempty"`
}

type Customize struct {
	CPUCores       int    `yaml:"cpu_cores,omitempty" json:"cpu_cores,omitempty"`
	MaxCapacity    int    `yaml:"max_capacity,omitempty" json:"max_capacity,omitempty"`
	OutputDir      string `yaml:"output_dir,omitempty" json:"output_dir,omitempty"`
	RawFilename    string `yaml:"raw_filename,omitempty" json:"raw_filename,omitempty"`
	DomainFilename string `yaml:"domain_filename,omitempty" json:"domain_filename,omitempty"`
	IPFilename     string `yaml:"ip_filename,omitempty" json:"ip_filename,omitempty"`
}

// RawConfig defines configuration from config files
type RawConfig struct {
	*Crawl
	*Filter
	*Customize
}

func (r *RawConfig) ParseRawConfig(configFile string) error {
	switch {
	case strings.HasSuffix(configFile, ".yaml"), strings.HasSuffix(configFile, ".yml"):
		configBytes, err := os.ReadFile(configFile)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(configBytes, &r); err != nil {
			return err
		}
	case strings.HasSuffix(configFile, ".json"):
		configBytes, err := json.Marshal(configFile)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(configBytes, &r); err != nil {
			return err
		}
	default:
		return ErrConfigFormatNotSupported
	}
	return nil
}

// GreatFireURL defines the structure of the format of URL
type GreatFireURL struct {
	BaseURL       string
	TypeURL       string
	SuffixURL     string
	InitSuffixURL string
}

// CrawlType defines the structure of AlexaTop1000 type of URLs and list
type CrawlType struct {
	*GreatFireURL
	Name         string
	IsCrawl      bool
	MaxPage      int
	From, To     int
	InitElement  *Elem
	CrawlElement *Elem
	CrawlReferer string
	CrawlList    []string
}

// Config defines the real configuration used in the program
type Config struct {
	*Filter
	*Customize
	Types []*CrawlType
}

// GenerateConfig generates raw config to config that can be used in the program
func (c *Config) GenerateConfig(r *RawConfig) error {
	if r != nil {
		if r.Filter != nil {
			c.Filter = r.Filter
		} else {
			return ErrFilterConfigIsEmpty
		}
		if r.Customize != nil {
			c.Customize = r.Customize
		} else {
			return ErrCustomizeConfigIsEmpty
		}
		if r.Crawl != nil && r.Crawl.Types != nil {
			c.Types = make([]*CrawlType, len(r.Crawl.Types))
			for i := 0; i < len(r.Crawl.Types); i++ {
				rawType := r.Crawl.Types[i]
				c.Types[i] = &CrawlType{
					GreatFireURL: &GreatFireURL{
						BaseURL:       r.Crawl.URL.BaseURL,
						TypeURL:       rawType.TypeURL,
						SuffixURL:     r.Crawl.URL.SuffixURL,
						InitSuffixURL: r.Crawl.URL.InitSuffixURL,
					},
					Name:         rawType.Name,
					IsCrawl:      rawType.IsCrawl,
					From:         rawType.From,
					To:           rawType.To,
					InitElement:  r.Crawl.InitElement,
					CrawlElement: r.Crawl.CrawlElement,
					CrawlReferer: rawType.Referer,
				}
			}
			return nil
		} else {
			return ErrCrawlConfigIsEmpty
		}
	}
	return ErrConfigIsEmpty
}

// SetNumCPU sets the maximum number of Goroutines
func (c *Config) SetNumCPU() error {
	if c.Customize != nil {
		setNum := c.Customize.CPUCores
		originalNumCPU := runtime.NumCPU()
		log.Println("Original CPU cores:", originalNumCPU)

		if setNum > originalNumCPU {
			runtime.GOMAXPROCS(setNum)
			log.Println("Now CPU cores:", setNum)
			return nil
		}
		switch {
		case originalNumCPU == 1:
			originalNumCPU = 3
		case originalNumCPU == 2:
			originalNumCPU *= 3
		case originalNumCPU == 3:
			originalNumCPU *= 2
		case originalNumCPU == 4:
			originalNumCPU = 10
		default:
			originalNumCPU += int(0.5 * float64(originalNumCPU))
		}
		runtime.GOMAXPROCS(originalNumCPU)
		c.Customize.CPUCores = originalNumCPU
		log.Println("Now CPU cores:", originalNumCPU)
		return nil
	} else {
		return ErrCustomizeConfigIsEmpty
	}
}

// CrawlMaxPage gets the max page of crawl type
func (c *Config) CrawlMaxPage() chan error {
	var wg sync.WaitGroup
	wg.Add(len(c.Types))

	e := make(chan error, len(c.Types))
	for idx, crawlType := range c.Types {
		go func(idx int, crawlType *CrawlType) {
			crawlInitURL := crawlType.BaseURL + crawlType.TypeURL + crawlType.InitSuffixURL
			crawlName := crawlType.Name
			crawlContent := crawlType.InitElement.Content

			switch crawlType.IsCrawl {
			case false:
				log.Printf("Type %s has been disabled to crawl.\n", crawlName)
			default:
				resp, err := crawler.Crawl(crawlInitURL, crawlType.CrawlReferer)
				if err != nil {
					e <- err
					return
				}
				defer resp.Body.Close()

				gzipReader, err := gzip.NewReader(resp.Body)
				if err != nil {
					e <- err
					return
				}
				defer gzipReader.Close()

				// Load the HTML document
				doc, err := goquery.NewDocumentFromReader(gzipReader)
				if err != nil {
					e <- err
					return
				}

				// Find items
				doc.Find(crawlType.InitElement.Container).Each(func(i int, s *goquery.Selection) {
					// For each item found, get contents
					if lastPageHref, exists := s.Find(crawlContent).Attr(crawlType.InitElement.Attr); !exists {
						log.Printf("Cannot find HTML element `%s`\n", crawlContent)
					} else {
						matchedSlice := strings.Split(lastPageHref, crawlType.InitElement.Splitter)
						if len(matchedSlice) == 2 {
							maxPageString := matchedSlice[1]
							if maxpage, err := strconv.Atoi(maxPageString); err != nil {
								log.Printf("Failed to get max page of type %s.\n", crawlName)
							} else {
								c.Types[idx].MaxPage = maxpage
								log.Printf("Type %s has pages: %d\n", crawlName, maxpage+1)
							}
						}
					}
				})
			}
			wg.Done()
		}(idx, crawlType)
	}

	wg.Wait()
	defer close(e)
	return e
}

// GenerateCrawlList generates lists for each crawl type to be crawled latter
func (c *Config) GenerateCrawlList() error {
	for idx, crawlType := range c.Types {
		if !crawlType.IsCrawl {
			continue
		}
		maxpage := crawlType.MaxPage
		from := crawlType.From
		to := crawlType.To

		if to < 0 {
			to = maxpage
		}

		if from < 0 || from > maxpage || to > maxpage || from > to {
			return ErrInvalidPageNumber
		}

		log.Printf("Type %s will be crawled from page %d to %d", crawlType.Name, from, to)

		list := make([]string, 0, maxpage)
		for i := from; i <= to; i++ {
			url := crawlType.BaseURL + crawlType.TypeURL + crawlType.SuffixURL + strconv.Itoa(i)
			list = append(list, url)
		}

		c.Types[idx].CrawlList = list
	}
	return nil
}

// Crawl gets HTML content for crawl types
func (c *Config) Crawl(rawResultChan chan map[*string]int) {
	var wg sync.WaitGroup
	workerPool := make(chan struct{}, c.Customize.CPUCores)

	for _, crawlType := range c.Types {
		for _, url := range crawlType.CrawlList {
			workerPool <- struct{}{}
			wg.Add(1)

			go func(url string, crawlType *CrawlType) {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("Goroutine panic: fetching %v : %v\n", url, err)
					}
				}()

				container := crawlType.CrawlElement.Container
				content := crawlType.CrawlElement.Content
				attr := crawlType.CrawlElement.Attr
				condition := crawlType.CrawlElement.Condition

				log.Println("Crawling:", url)
				resp, err := crawler.Crawl(url, crawlType.CrawlReferer)
				utils.Must(err)
				defer resp.Body.Close()

				gzipReader, err := gzip.NewReader(resp.Body)
				utils.Must(err)
				defer gzipReader.Close()

				// Load the HTML document
				doc, err := goquery.NewDocumentFromReader(gzipReader)
				utils.Must(err)

				// Find items
				doc.Find(container).Each(func(i int, s *goquery.Selection) {
					percent := 0
					// For each item found, get contents
					rawDomain, _ := s.Find(content).Attr(attr)
					if blockedPercentage := strings.TrimSpace(s.Find(condition).Text()); blockedPercentage != "" {
						percent, _ = strconv.Atoi(blockedPercentage[:len(blockedPercentage)-1])
					}

					rawResult := make(map[*string]int)
					rawResult[&rawDomain] = percent
					rawResultChan <- rawResult
				})

				wg.Done()
				<-workerPool
			}(url, crawlType)
		}
	}

	wg.Wait()
	close(rawResultChan)
}

// FilterAndWrite filters HTML conent and write results to files
func (c *Config) FilterAndWrite(rawResultChan chan map[*string]int) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Runtime panic: %v\n", err)
		}
	}()

	// Make output dir
	utils.Must(os.MkdirAll(filepath.Join("./", c.Customize.OutputDir), 0755))

	rawDomainFile, err := os.OpenFile(filepath.Join(c.Customize.OutputDir, c.Customize.RawFilename), os.O_WRONLY|os.O_CREATE, 0644)
	utils.Must(err)
	defer rawDomainFile.Close()

	finalDomainFile, err := os.OpenFile(filepath.Join(c.Customize.OutputDir, c.Customize.DomainFilename), os.O_WRONLY|os.O_CREATE, 0644)
	utils.Must(err)
	defer finalDomainFile.Close()

	finalIPfile, err := os.OpenFile(filepath.Join(c.Customize.OutputDir, c.Customize.IPFilename), os.O_WRONLY|os.O_CREATE, 0644)
	utils.Must(err)
	defer finalIPfile.Close()

	resultMap := make(map[string]struct{})
	domainReg := regexp.MustCompile(c.Filter.Regexp.Domain)
	rawReader := bufio.NewWriter(rawDomainFile)
	for result := range rawResultChan {
		for url, percent := range result {
			url := strings.ToLower(*url)
			// Write raw results to raw.txt file
			rawReader.WriteString(fmt.Sprintf("%s | %d\n", url, percent))

			if percent >= c.Filter.Percent {
				matchList := domainReg.FindStringSubmatch(url)
				if len(matchList) > 0 {
					domain := matchList[len(matchList)-2]
					// Write filtered results to console
					fmt.Printf("%s | %d\n", domain, percent)
					// Write filtered results to map to make them unique
					resultMap[domain] = struct{}{}
				}
			}
		}
	}
	rawReader.Flush()

	resultSlice := make([]string, 0, len(resultMap))
	ipSlice := make([]string, 0, len(resultMap))
	ipReg := regexp.MustCompile(c.Filter.Regexp.IP)
	for domainOrIP := range resultMap {
		ipElem := ipReg.FindStringSubmatch(domainOrIP)
		if len(ipElem) > 0 {
			ipSlice = append(ipSlice, ipElem[0])
			continue
		}
		resultSlice = append(resultSlice, domainOrIP)
	}

	// Unique and sort domain slice
	sort.SliceStable(resultSlice, func(i, j int) bool {
		return len(strings.Split(resultSlice[i], ".")) < len(strings.Split(resultSlice[j], "."))
	})
	resultSlice = buildTreeAndUnique(resultSlice)
	sort.Strings(resultSlice)

	// Write filtered result to domains.txt file
	domainReader := bufio.NewWriter(finalDomainFile)
	for _, domain := range resultSlice {
		domainReader.WriteString(fmt.Sprintf("%s\n", domain))
	}
	domainReader.Flush()

	// Sort IP slice
	sort.Strings(ipSlice)

	// Write IP results to ip.txt file
	ipReader := bufio.NewWriter(finalIPfile)
	for _, ip := range ipSlice {
		ipReader.WriteString(fmt.Sprintf("%s\n", ip))
	}
	ipReader.Flush()
}
