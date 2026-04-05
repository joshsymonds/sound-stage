package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/joshsymonds/sound-stage/usdb"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search USDB for songs",
	Long:  `Search usdb.animux.de by artist, title, or both. Use --artist and --title for specific fields, or pass a free-text query.`,
	Args:  cobra.ArbitraryArgs,
	RunE:  runSearch,
}

var (
	searchArtist string
	searchTitle  string
	searchLimit  int
)

func init() {
	searchCmd.Flags().StringVarP(&searchArtist, "artist", "a", "", "filter by artist")
	searchCmd.Flags().StringVarP(&searchTitle, "title", "t", "", "filter by title")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 25, "max results to show")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(_ *cobra.Command, _ []string) error {
	if err := requireCredentials(); err != nil {
		return err
	}

	client, err := usdb.NewClient(username, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	results, err := client.Search(usdb.SearchParams{
		Artist: searchArtist,
		Title:  searchTitle,
		Limit:  searchLimit,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tARTIST\tTITLE\tLANGUAGE")
	for _, s := range results {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", s.ID, s.Artist, s.Title, s.Language)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushing output: %w", err)
	}

	return nil
}
