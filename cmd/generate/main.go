package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

// ── Color type ─────────────────────────────────────────────────────────────

// Color unmarshals from YAML as either a bare 0xRRGGBB[AA] integer literal
// (no quotes needed) or any CSS color string.  0x notation is converted to
// the #RRGGBB[AA] form expected by CSS.
type Color string

func (c *Color) UnmarshalYAML(value *yaml.Node) error {
	raw := value.Value
	if strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X") {
		*c = Color("#" + raw[2:])
	} else {
		*c = Color(raw)
	}
	return nil
}

func (c Color) IsZero() bool { return c == "" }

// ── Raw config types (YAML) ────────────────────────────────────────────────

type titleCfg struct {
	Text  string `yaml:"text"`
	Font  string `yaml:"font"`
	Style string `yaml:"style"` // CSS font-style, e.g. "italic"
	FG    Color  `yaml:"fg"`
	Size  string `yaml:"size"` // keyword (large/medium/small) or any CSS value
}

type shortCfg struct {
	Text string `yaml:"text"`
	Font string `yaml:"font"`
	FG   Color  `yaml:"fg"`
	Size string `yaml:"size"`
}

type descCfg struct {
	Text string `yaml:"text"` // path to .md file (required)
	FG   Color  `yaml:"fg"`   // required
	BG   Color  `yaml:"bg"`   // required
}

// imageCfg accepts either a plain path string or a mapping with path/size/position.
//
//	image: ./foo.webp                     # plain string
//	image:
//	  path: ./foo.webp
//	  size: contain                       # any CSS background-size value
//	  position: top center               # any CSS background-position value
type imageCfg struct {
	Path     string // resolved source path
	Size     string // CSS background-size, default "cover"
	Position string // CSS background-position, default "center"
}

func (ic *imageCfg) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag == "!!str" {
		ic.Path = strings.TrimSpace(value.Value)
		return nil
	}
	var m struct {
		Path     string `yaml:"path"`
		Size     string `yaml:"size"`
		Position string `yaml:"position"`
	}
	if err := value.Decode(&m); err != nil {
		return err
	}
	ic.Path = strings.TrimSpace(m.Path)
	ic.Size = m.Size
	ic.Position = m.Position
	return nil
}

type rawConfig struct {
	Title titleCfg `yaml:"title"`
	Short shortCfg `yaml:"short"`
	Image imageCfg `yaml:"image"`
	Desc  descCfg  `yaml:"description"`
	URL   string   `yaml:"url"`
	Order int      `yaml:"order"`
	Theme string   `yaml:"theme"` // "light" or "dark" (default "dark")
}

// ── Template types ─────────────────────────────────────────────────────────

// Repo holds all computed values needed to render one slide.
type Repo struct {
	DirName       string
	Order         int
	URL           string
	Theme         string // "light" or "dark"
	ImagePath     string // destination path within site/, e.g. "img/myproject.webp"
	ImageSize     string // CSS background-size
	ImagePosition string // CSS background-position
	imageSrc      string // full source path on disk
	TitleText     string
	TitleStyle    template.CSS // inline CSS applied to the <h2>
	ShortText     string
	ShortStyle    template.CSS // inline CSS applied to the <p>
	DescFG        string
	DescBG        string
	DescHTML      template.HTML
	tempDir       string // non-empty if fetched from a remote repo; cleaned up after build
}

// descItem is per-slide description data embedded as JSON for carousel.js.
type descItem struct {
	FG   string `json:"fg"`
	BG   string `json:"bg"`
	HTML string `json:"html"`
}

// TemplateData is the top-level value passed to index.html.tmpl.
type TemplateData struct {
	Repos    []Repo
	DescJSON template.JS
}

// ── Helpers ────────────────────────────────────────────────────────────────

var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".svg": true,
}

var fontStack = map[string]string{
	"georgia":   `Georgia, "Times New Roman", serif`,
	"times":     `Times, "Times New Roman", serif`,
	"helvetica": `Helvetica, Arial, sans-serif`,
	"arial":     `Arial, Helvetica, sans-serif`,
	"verdana":   `Verdana, Geneva, sans-serif`,
	"courier":   `"Courier New", Courier, monospace`,
}

var sizeKeywords = map[string]string{
	"small":  "clamp(1.8rem, 4vw, 3rem)",
	"medium": "clamp(2.5rem, 6vw, 4.5rem)",
	"large":  "clamp(3.5rem, 10vw, 7rem)",
	"xlarge": "clamp(4.5rem, 13vw, 9rem)",
}

// resolveFont maps short names (georgia, times, …) to full CSS font stacks.
func resolveFont(name, fallback string) string {
	if name == "" {
		return fallback
	}
	if fam, ok := fontStack[strings.ToLower(strings.TrimSpace(name))]; ok {
		return fam
	}
	return name
}

// resolveSize maps size keywords to clamp() values; passes CSS values through.
func resolveSize(s, fallback string) string {
	if s == "" {
		return fallback
	}
	if mapped, ok := sizeKeywords[strings.ToLower(strings.TrimSpace(s))]; ok {
		return mapped
	}
	return s
}

func orStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func renderMarkdown(s string) template.HTML {
	if s == "" {
		return ""
	}
	var b strings.Builder
	if err := md.Convert([]byte(s), &b); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}
	return template.HTML(strings.TrimSpace(b.String()))
}

// isWithinDir reports whether absPath is inside (or equal to) dirPath.
func isWithinDir(dirPath, absPath string) bool {
	rel, err := filepath.Rel(dirPath, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// displayPath returns a concise path for logging.
// If path lives inside repoRoot it is shown relative to repoRoot;
// otherwise it is shown relative to the current working directory.
func displayPath(path, repoRoot string) string {
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	if rel, err := filepath.Rel(".", path); err == nil {
		return rel
	}
	return path
}

// ── Remote repo fetching ───────────────────────────────────────────────────

// fetchRemoteRepo does a depth-1 sparse clone of repoURL, checking out only
// the .tivoli/ directory.  Returns the path to the temp dir (the repo root).
// The caller is responsible for os.RemoveAll when done.
func fetchRemoteRepo(repoURL string) (string, error) {
	tmp, err := os.MkdirTemp("", "tivoli-*")
	if err != nil {
		return "", fmt.Errorf("mktemp: %w", err)
	}
	run := func(args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := run("git", "clone", "--depth=1", "--filter=blob:none",
		"--no-checkout", "--quiet", repoURL, tmp); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("git clone: %w", err)
	}
	if err := run("git", "-C", tmp, "sparse-checkout", "init", "--cone"); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("git sparse-checkout init: %w", err)
	}
	if err := run("git", "-C", tmp, "sparse-checkout", "set", ".tivoli"); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("git sparse-checkout set: %w", err)
	}
	if err := run("git", "-C", tmp, "checkout"); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("git checkout: %w", err)
	}
	return tmp, nil
}

// expandSparseCone adds extra directories to the sparse checkout and
// re-materialises them so that files outside .tivoli/ can be read.
func expandSparseCone(repoDir string, dirs []string) error {
	args := append([]string{"-C", repoDir, "sparse-checkout", "add"}, dirs...)
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ── Loading ────────────────────────────────────────────────────────────────

func loadRepos(dir string) ([]Repo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var repos []Repo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		r, err := loadRepo(dir, e.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", e.Name(), err)
			continue
		}
		repos = append(repos, r)
	}
	sort.SliceStable(repos, func(i, j int) bool {
		oi, oj := repos[i].Order, repos[j].Order
		switch {
		case oi == 0 && oj == 0:
			return repos[i].DirName < repos[j].DirName
		case oi == 0:
			return false
		case oj == 0:
			return true
		default:
			return oi < oj
		}
	})
	return repos, nil
}

func loadRepo(base, name string) (Repo, error) {
	localDir := filepath.Join(base, name)

	cfgData, err := os.ReadFile(filepath.Join(localDir, "config.yaml"))
	if err != nil {
		return Repo{}, fmt.Errorf("reading config.yaml: %w", err)
	}
	var cfg rawConfig
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		return Repo{}, fmt.Errorf("parsing config.yaml: %w", err)
	}

	// ── Remote config resolution ──────────────────────────────────────────
	// A stub config has a url but no title.  We fetch .tivoli/config.yaml
	// from the upstream repo and use that instead.  The local config may
	// only contribute an order override.

	isRemote := false
	localOrder := cfg.Order // preserved for possible override
	var tempDir string
	dir := localDir // base dir for resolving relative paths in config

	if cfg.Title.Text == "" {
		if cfg.URL == "" {
			return Repo{}, fmt.Errorf("config.yaml: url is required when no title is set")
		}
		tmp, fetchErr := fetchRemoteRepo(cfg.URL)
		if fetchErr != nil {
			return Repo{}, fmt.Errorf("fetching remote repo: %w", fetchErr)
		}
		tivoliDir := filepath.Join(tmp, ".tivoli")
		remoteCfgPath := filepath.Join(tivoliDir, "config.yaml")
		if _, statErr := os.Stat(remoteCfgPath); statErr != nil {
			os.RemoveAll(tmp)
			return Repo{}, fmt.Errorf("upstream repo has no .tivoli/config.yaml")
		}
		remoteCfgData, err := os.ReadFile(remoteCfgPath)
		if err != nil {
			os.RemoveAll(tmp)
			return Repo{}, fmt.Errorf("reading remote .tivoli/config.yaml: %w", err)
		}
		var remoteCfg rawConfig
		if err := yaml.Unmarshal(remoteCfgData, &remoteCfg); err != nil {
			os.RemoveAll(tmp)
			return Repo{}, fmt.Errorf("parsing remote .tivoli/config.yaml: %w", err)
		}
		remoteCfg.URL = cfg.URL // stub URL is authoritative
		if localOrder != 0 {
			remoteCfg.Order = localOrder
		}
		cfg = remoteCfg
		dir = tivoliDir
		tempDir = tmp
		isRemote = true
	}

	// ── Validation ────────────────────────────────────────────────────────

	if cfg.Title.Text == "" {
		return Repo{}, fmt.Errorf("config.yaml: title.text is required")
	}
	if cfg.URL == "" {
		return Repo{}, fmt.Errorf("config.yaml: url is required")
	}
	if cfg.Desc.FG == "" || cfg.Desc.BG == "" {
		return Repo{}, fmt.Errorf("config.yaml: description.fg and description.bg are required")
	}

	// ── Sparse-checkout expansion for external references ─────────────────
	// Config paths (image, description) may use relative paths that escape
	// .tivoli/ (e.g. ../docs/summary.md).  Expand the sparse cone to include
	// those directories before we try to read the files.

	if isRemote {
		var extraDirs []string
		checkPath := func(rawPath string) {
			if rawPath == "" {
				return
			}
			abs := rawPath
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(dir, rawPath)
			}
			abs = filepath.Clean(abs)
			if !isWithinDir(dir, abs) {
				if rel, err := filepath.Rel(tempDir, filepath.Dir(abs)); err == nil {
					extraDirs = append(extraDirs, rel)
				}
			}
		}
		checkPath(cfg.Image.Path)
		checkPath(cfg.Desc.Text)
		if len(extraDirs) > 0 {
			if err := expandSparseCone(tempDir, extraDirs); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: expanding sparse checkout: %v\n", name, err)
			}
		}
	}

	// ── Style computation ─────────────────────────────────────────────────

	titleFG := string(cfg.Title.FG)
	if titleFG == "" {
		titleFG = "#ffffff"
	}
	shortFG := string(cfg.Short.FG)
	if shortFG == "" {
		shortFG = titleFG
	}
	descFG := string(cfg.Desc.FG)
	descBG := string(cfg.Desc.BG)

	const defaultFont = `Georgia, "Times New Roman", serif`
	titleFont := resolveFont(cfg.Title.Font, defaultFont)
	shortFont := resolveFont(cfg.Short.Font, titleFont)

	titleStyle := template.CSS(fmt.Sprintf(
		"font-family: %s; font-style: %s; color: %s; font-size: %s",
		titleFont,
		orStr(cfg.Title.Style, "normal"),
		titleFG,
		resolveSize(cfg.Title.Size, "clamp(3rem, 9vw, 6rem)"),
	))
	shortStyle := template.CSS(fmt.Sprintf("font-family: %s; color: %s; font-size: %s",
		shortFont, shortFG,
		resolveSize(cfg.Short.Size, "clamp(1.2rem, 3vw, 1.6rem)"),
	))

	// ── Description ───────────────────────────────────────────────────────

	var descHTML template.HTML
	var descSrc string // for logging
	if cfg.Desc.Text != "" {
		descPath := strings.TrimSpace(cfg.Desc.Text)
		if !filepath.IsAbs(descPath) {
			descPath = filepath.Join(dir, descPath)
		}
		descPath = filepath.Clean(descPath)
		descSrc = descPath
		if raw, readErr := os.ReadFile(descPath); readErr == nil {
			descHTML = renderMarkdown(strings.TrimSpace(string(raw)))
		} else {
			fmt.Fprintf(os.Stderr, "warning: %s: reading description %s: %v\n", name, cfg.Desc.Text, readErr)
		}
	}

	// ── Theme ─────────────────────────────────────────────────────────────

	theme := cfg.Theme
	if theme != "light" && theme != "dark" {
		theme = "dark"
	}

	// ── Image resolution ──────────────────────────────────────────────────

	r := Repo{
		DirName:    name,
		Order:      cfg.Order,
		URL:        cfg.URL,
		Theme:      theme,
		TitleText:  cfg.Title.Text,
		TitleStyle: titleStyle,
		ShortText:  cfg.Short.Text,
		ShortStyle: shortStyle,
		DescFG:     descFG,
		DescBG:     descBG,
		DescHTML:   descHTML,
		tempDir:    tempDir,
	}

	r.ImageSize = orStr(cfg.Image.Size, "cover")
	r.ImagePosition = orStr(cfg.Image.Position, "center")

	if cfg.Image.Path != "" {
		imgPath := cfg.Image.Path
		if !filepath.IsAbs(imgPath) {
			imgPath = filepath.Join(dir, imgPath)
		}
		imgPath = filepath.Clean(imgPath)
		if _, statErr := os.Stat(imgPath); statErr == nil {
			ext := strings.ToLower(filepath.Ext(imgPath))
			r.imageSrc = imgPath
			r.ImagePath = "img/" + name + ext
		}
	}
	if r.imageSrc == "" {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if imageExts[ext] {
				r.imageSrc = filepath.Join(dir, e.Name())
				r.ImagePath = "img/" + name + ext
				break
			}
		}
	}
	if r.imageSrc == "" {
		if isRemote {
			os.RemoveAll(tempDir)
		}
		return Repo{}, fmt.Errorf("no image found (set image: or add an image file)")
	}

	// ── Source log ────────────────────────────────────────────────────────

	repoRoot := "" // used by displayPath to make paths relative to the upstream repo
	if isRemote {
		repoRoot = tempDir
	}

	if isRemote {
		fmt.Printf("%s  ← remote (%s)\n", name, cfg.URL)
	} else {
		fmt.Printf("%s  ← local\n", name)
	}
	fmt.Printf("  config:  %s\n", displayPath(filepath.Join(dir, "config.yaml"), repoRoot))
	fmt.Printf("  image:   %s\n", displayPath(r.imageSrc, repoRoot))
	if descSrc != "" {
		fmt.Printf("  desc:    %s\n", displayPath(descSrc, repoRoot))
	}
	if isRemote && localOrder != 0 {
		fmt.Printf("  order:   %d (local)\n", localOrder)
	}
	fmt.Println()

	return r, nil
}

// ── Main ───────────────────────────────────────────────────────────────────

func main() {
	repos, err := loadRepos("repos")
	if err != nil {
		die("loading repos: %v", err)
	}
	if len(repos) == 0 {
		die("no repos found in repos/ directory")
	}

	if err := os.MkdirAll("site/img", 0o755); err != nil {
		die("creating site/img: %v", err)
	}

	for i, r := range repos {
		if r.imageSrc == "" {
			continue
		}
		dst := filepath.Join("site", r.ImagePath)
		if err := copyFile(r.imageSrc, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping image for %s: %v\n", r.DirName, err)
			repos[i].ImagePath = ""
		}
	}

	// Clean up any temp dirs from remote repos now that images are copied.
	for _, r := range repos {
		if r.tempDir != "" {
			os.RemoveAll(r.tempDir)
		}
	}

	if err := copyDir("static", "site"); err != nil {
		die("copying static files: %v", err)
	}

	// Build per-slide JSON blob for carousel.js.
	items := make([]descItem, len(repos))
	for i, r := range repos {
		items[i] = descItem{FG: r.DescFG, BG: r.DescBG, HTML: string(r.DescHTML)}
	}
	descJSONBytes, err := json.Marshal(items)
	if err != nil {
		die("marshaling desc data: %v", err)
	}

	tmpl, err := template.New("index.html.tmpl").ParseFiles("templates/index.html.tmpl")
	if err != nil {
		die("parsing template: %v", err)
	}

	f, err := os.Create("site/index.html")
	if err != nil {
		die("creating site/index.html: %v", err)
	}
	defer f.Close()

	data := TemplateData{
		Repos:    repos,
		DescJSON: template.JS(descJSONBytes),
	}
	if err := tmpl.Execute(f, data); err != nil {
		die("rendering template: %v", err)
	}

	fmt.Printf("generated site/ with %d repo(s)\n", len(repos))
}

// ── File utils ─────────────────────────────────────────────────────────────

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
