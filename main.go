package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/Loyalsoldier/cn-blocked-domain/crawler"
)

func run(ctx context.Context, args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("cn-blocked-domain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	maxParallel := flags.Int("max-parallel", 10, "Maximum simultaneous requests (at least 1)")
	outputDir := flags.String("output-dir", "publish", "Directory for generated lists")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*outputDir) == "" {
		return fmt.Errorf("output directory must not be empty")
	}
	client, err := crawler.New(*maxParallel)
	if err != nil {
		return err
	}
	values, err := client.Fetch(ctx)
	if err != nil {
		return err
	}
	lists := aggregate(values)
	if err := writeLists(*outputDir, lists); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "Wrote %d domains, %d removed domains and %d IP networks to %s; skipped %d invalid values\n",
		len(lists.domains), len(lists.removed), len(lists.prefixes), *outputDir, lists.invalid)
	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stderr); err != nil {
		log.Fatal(err)
	}
}
