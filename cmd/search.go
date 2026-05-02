package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/joshsymonds/sound-stage/usdb"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search USDB for songs",
	Long:  `Search usdb.animux.de by artist, title, or both. Use --artist and --title for specific fields.`,
	RunE:  runSearch,
}

var (
	searchArtist  string
	searchTitle   string
	searchEdition string
	searchLimit   int
	searchJSON    bool
)

// osStdout is the writer used for JSON and table output. Tests can replace it.
var osStdout io.Writer = os.Stdout

func init() {
	searchCmd.Flags().StringVarP(&searchArtist, "artist", "a", "", "filter by artist")
	searchCmd.Flags().StringVarP(&searchTitle, "title", "t", "", "filter by title")
	searchCmd.Flags().StringVarP(
		&searchEdition, "edition", "e", "", "filter by edition",
	)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 25, "max results to show")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "output results as JSON")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, _ []string) error {
	if err := requireCredentials(); err != nil {
		return err
	}

	client, err := usdb.NewClient(username, password)
	if err != nil {
		return fmt.Errorf("creating USDB client: %w", err)
	}
	if err := client.Login(cmd.Context()); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	results, err := client.Search(usdb.SearchParams{
		Artist:  searchArtist,
		Title:   searchTitle,
		Edition: searchEdition,
		Limit:   searchLimit,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if searchJSON {
		return outputJSON(results)
	}

	return outputTable(results)
}

func outputJSON(results []usdb.Song) error {
	enc := json.NewEncoder(osStdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(results); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	return nil
}

func outputTable(results []usdb.Song) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	w := tabwriter.NewWriter(osStdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tARTIST\tTITLE\tLANGUAGE")

	for _, s := range results {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", s.ID, s.Artist, s.Title, s.Language)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushing output: %w", err)
	}

	return nil
}
