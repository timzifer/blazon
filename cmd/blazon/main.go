// Command blazon renders version numbers as visual seals, and provides the
// tools used to judge whether a renderer actually distinguishes them.
//
// The dist and sheet subcommands are not extras: calibrating a renderer's
// thresholds means looking at its distance histogram and at a contact sheet,
// so both belong in the shipped tool rather than in a test helper.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/timzifer/blazon"
	"github.com/timzifer/blazon/blazontest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "blazon: "+err.Error())
		os.Exit(1)
	}
}

const usage = `blazon — version numbers as visual seals

usage:
  blazon [flags] <version>       render a version
  blazon list                    list the renderers
  blazon version                 print the library version
  blazon dist [flags]            print the distance histogram for a renderer
  blazon sheet [flags]           write a contact sheet of a version corpus
  blazon gallery [flags]         regenerate the gallery images and the README

render flags:
  -r name        renderer (default %q)
  -o file        write to file; the extension picks the format (.svg, .png)
                 omit to write terminal output to stdout
  -size n        edge length in pixels for -o output (default 512)
  -cols n        terminal width in character cells (default 32)
  -policy p      family-amplified (default), family, independent
  -palette p     color (default), mono
  -bg b          transparent (default), light, dark
  -no-color      terminal output without ANSI colour

run "blazon <subcommand> -h" for the flags of a subcommand.
`

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(fmt.Sprintf(usage, blazon.DefaultRenderer))
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println(blazon.Release)
		return nil
	case "list":
		return cmdList()
	case "dist":
		return cmdDist(args[1:])
	case "sheet":
		return cmdSheet(args[1:])
	case "gallery":
		return cmdGallery(args[1:])
	case "-h", "--help", "help":
		fmt.Print(fmt.Sprintf(usage, blazon.DefaultRenderer))
		return nil
	}
	return cmdRender(args)
}

// commonFlags registers the options shared by the rendering subcommands.
type commonFlags struct {
	renderer string
	size     int
	cols     int
	policy   string
	palette  string
	bg       string
	noColor  bool
}

func (c *commonFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&c.renderer, "r", blazon.DefaultRenderer, "renderer name")
	fs.IntVar(&c.size, "size", 512, "edge length in pixels")
	fs.IntVar(&c.cols, "cols", 32, "terminal width in character cells")
	fs.StringVar(&c.policy, "policy", "family-amplified", "family-amplified, family or independent")
	fs.StringVar(&c.palette, "palette", "color", "color or mono")
	fs.StringVar(&c.bg, "bg", "transparent", "transparent, light or dark")
	fs.BoolVar(&c.noColor, "no-color", false, "terminal output without ANSI colour")
}

func (c *commonFlags) options() (blazon.Options, error) {
	o := blazon.Options{
		Renderer: c.renderer,
		Size:     c.size,
		Cols:     c.cols,
		NoColor:  c.noColor || os.Getenv("NO_COLOR") != "",
	}
	switch c.policy {
	case "family-amplified", "":
		o.Policy = blazon.PolicyFamilyAmplified
	case "family":
		o.Policy = blazon.PolicyFamily
	case "independent":
		o.Policy = blazon.PolicyIndependent
	default:
		return o, fmt.Errorf("unknown policy %q", c.policy)
	}
	switch c.palette {
	case "color", "":
		o.Palette = blazon.PaletteColor
	case "mono":
		o.Palette = blazon.PaletteMono
	default:
		return o, fmt.Errorf("unknown palette %q", c.palette)
	}
	switch c.bg {
	case "transparent", "":
		o.Background = blazon.BackgroundTransparent
	case "light":
		o.Background = blazon.BackgroundLight
	case "dark":
		o.Background = blazon.BackgroundDark
	default:
		return o, fmt.Errorf("unknown background %q", c.bg)
	}
	return o, nil
}

func cmdList() error {
	for _, name := range blazon.Renderers() {
		r, _ := blazon.Lookup(name)
		caps := r.Caps()
		var tags []string
		if caps.TextOnly {
			tags = append(tags, fmt.Sprintf("terminal %dx%d", caps.GridCols, caps.GridRows))
		}
		if caps.ByteExact {
			tags = append(tags, "byte-exact")
		}
		if caps.Ordered {
			tags = append(tags, "ordered")
		}
		if name == blazon.DefaultRenderer {
			tags = append(tags, "default")
		}
		if len(tags) > 0 {
			fmt.Printf("%-14s %s\n", name, strings.Join(tags, ", "))
			continue
		}
		fmt.Println(name)
	}
	return nil
}

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("blazon", flag.ContinueOnError)
	var c commonFlags
	c.register(fs)
	out := fs.String("o", "", "output file; extension selects the format")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("expected exactly one version argument")
	}
	version := fs.Arg(0)

	o, err := c.options()
	if err != nil {
		return err
	}

	if *out == "" {
		s, err := blazon.Text(version, o)
		if err != nil {
			return err
		}
		fmt.Print(s)
		return nil
	}

	var data []byte
	switch strings.ToLower(filepath.Ext(*out)) {
	case ".svg":
		data, err = blazon.SVG(version, o)
	case ".png":
		data, err = blazon.PNG(version, o)
	default:
		return fmt.Errorf("cannot tell the format of %q: use a .svg or .png extension", *out)
	}
	if err != nil {
		return err
	}
	return writeFile(*out, data)
}

func cmdDist(args []string) error {
	fs := flag.NewFlagSet("dist", flag.ContinueOnError)
	var c commonFlags
	c.register(fs)
	corpusPath := fs.String("corpus", "", "version corpus file (default: the built-in corpus)")
	all := fs.Bool("all", false, "report every renderer")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o, err := c.options()
	if err != nil {
		return err
	}

	// Unless the size was asked for explicitly, measure at the size the test
	// suite measures at. Reporting a histogram taken at some other resolution
	// would produce thresholds that the suite then fails to reproduce.
	sizeSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "size" {
			sizeSet = true
		}
	})
	if !sizeSet {
		o.Size = 0
	}

	corpus, err := loadCorpus(*corpusPath)
	if err != nil {
		return err
	}

	names := []string{c.renderer}
	if *all {
		names = blazon.Renderers()
	}
	for i, name := range names {
		r, ok := blazon.Lookup(name)
		if !ok {
			return fmt.Errorf("unknown renderer %q", name)
		}
		if i > 0 {
			fmt.Println()
		}
		rep, err := blazontest.Suite{Renderer: r, Options: o}.Report(corpus)
		if err != nil {
			return err
		}
		printReport(os.Stdout, rep)
	}
	return nil
}

func cmdSheet(args []string) error {
	fs := flag.NewFlagSet("sheet", flag.ContinueOnError)
	var c commonFlags
	c.register(fs)
	corpusPath := fs.String("corpus", "", "version corpus file (default: the built-in corpus)")
	out := fs.String("o", "sheet.png", "output PNG")
	cols := fs.Int("grid", 8, "marks per row")
	cell := fs.Int("cell", 128, "cell size in pixels")
	limit := fs.Int("n", 64, "how many versions to include; 0 for all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o, err := c.options()
	if err != nil {
		return err
	}
	r, ok := blazon.Lookup(c.renderer)
	if !ok {
		return fmt.Errorf("unknown renderer %q", c.renderer)
	}
	corpus, err := loadCorpus(*corpusPath)
	if err != nil {
		return err
	}
	if *limit > 0 && len(corpus) > *limit {
		corpus = corpus[:*limit]
	}
	img, err := blazontest.ContactSheet(r, corpus, o, *cols, *cell)
	if err != nil {
		return err
	}
	data, err := blazon.EncodePNG(img)
	if err != nil {
		return err
	}
	return writeFile(*out, data)
}

func loadCorpus(path string) ([]string, error) {
	if path == "" {
		return blazontest.DefaultCorpus()
	}
	return blazontest.LoadCorpus(path)
}

func writeFile(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}
