// Command cn-blocked-domain aggregates domains and IPs blocked in mainland China
// as reported by GreatFire Analyzer.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/Loyalsoldier/cn-blocked-domain/internal/domain"
	"github.com/Loyalsoldier/cn-blocked-domain/internal/greatfire"
	"github.com/Loyalsoldier/cn-blocked-domain/internal/ipaddr"
)

const (
	domainsFile      = "domains.txt"
	ipFile           = "ip.txt"
	deduplicatedFile = "deduplicated-domains.txt"
)

func main() {
	parallel := flag.Int("parallel", 10, "maximum number of pages to request at a time")
	outDir := flag.String("outdir", "publish", "directory to write output files to")
	flag.Parse()

	if *parallel < 1 {
		log.Fatal("-parallel must be at least 1")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, greatfire.NewClient(*parallel), *outDir); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, client *greatfire.Client, outDir string) error {
	entries, err := client.FetchAll(ctx)
	if err != nil {
		return err
	}
	log.Printf("fetched %d entries", len(entries))

	var domains []string
	var prefixes []netip.Prefix
	invalid := 0
	for _, e := range entries {
		if p, ok := ipaddr.Parse(e); ok {
			prefixes = append(prefixes, p)
		} else if d, ok := domain.Normalize(e); ok {
			domains = append(domains, d)
		} else {
			invalid++
		}
	}

	kept, removed := domain.Deduplicate(domains)
	cidrs := ipaddr.Aggregate(prefixes)
	log.Printf("domains: %d, deduplicated: %d, CIDRs: %d, invalid: %d", len(kept), len(removed), len(cidrs), invalid)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	ipLines := make([]string, len(cidrs))
	for i, c := range cidrs {
		ipLines[i] = c.String()
	}
	for name, lines := range map[string][]string{
		domainsFile:      kept,
		deduplicatedFile: removed,
		ipFile:           ipLines,
	} {
		if err := writeLines(filepath.Join(outDir, name), lines); err != nil {
			return err
		}
	}
	return nil
}

func writeLines(path string, lines []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}
