package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ── Color ──────────────────────────────────────────────────────────────────

func TestColorUnmarshal(t *testing.T) {
	cases := []struct {
		input string
		want  Color
	}{
		{"color: 0xffffff", "#ffffff"},
		{"color: 0xFF0000", "#FF0000"},
		{"color: 0xAABBCCDD", "#AABBCCDD"},
		{`color: "#ff0000"`, "#ff0000"},
		{"color: red", "red"},
	}
	for _, c := range cases {
		var s struct {
			Color Color `yaml:"color"`
		}
		if err := yaml.Unmarshal([]byte(c.input), &s); err != nil {
			t.Fatalf("%q: %v", c.input, err)
		}
		if s.Color != c.want {
			t.Errorf("%q: got %q, want %q", c.input, s.Color, c.want)
		}
	}
}

// ── imageCfg ───────────────────────────────────────────────────────────────

func TestImageCfgPlainString(t *testing.T) {
	var s struct {
		Image imageCfg `yaml:"image"`
	}
	if err := yaml.Unmarshal([]byte("image: ./screenshot.webp"), &s); err != nil {
		t.Fatal(err)
	}
	if s.Image.Path != "./screenshot.webp" {
		t.Errorf("path: got %q", s.Image.Path)
	}
	if s.Image.Size != "" || s.Image.Position != "" {
		t.Errorf("unexpected size/position: %q %q", s.Image.Size, s.Image.Position)
	}
}

func TestImageCfgMapping(t *testing.T) {
	input := "image:\n  path: ./screenshot.webp\n  size: contain\n  position: top center"
	var s struct {
		Image imageCfg `yaml:"image"`
	}
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatal(err)
	}
	if s.Image.Path != "./screenshot.webp" {
		t.Errorf("path: got %q", s.Image.Path)
	}
	if s.Image.Size != "contain" {
		t.Errorf("size: got %q", s.Image.Size)
	}
	if s.Image.Position != "top center" {
		t.Errorf("position: got %q", s.Image.Position)
	}
}

// ── resolveFont ────────────────────────────────────────────────────────────

func TestResolveFont(t *testing.T) {
	cases := []struct {
		name, fallback, want string
	}{
		{"georgia", "", `Georgia, "Times New Roman", serif`},
		{"times", "", `Times, "Times New Roman", serif`},
		{"GEORGIA", "", `Georgia, "Times New Roman", serif`},
		{"", "my-fallback", "my-fallback"},
		{"unknown-font", "", "unknown-font"},
	}
	for _, c := range cases {
		got := resolveFont(c.name, c.fallback)
		if got != c.want {
			t.Errorf("resolveFont(%q, %q): got %q, want %q", c.name, c.fallback, got, c.want)
		}
	}
}

// ── resolveSize ────────────────────────────────────────────────────────────

func TestResolveSize(t *testing.T) {
	cases := []struct {
		size, fallback, want string
	}{
		{"small", "", "clamp(1.8rem, 4vw, 3rem)"},
		{"medium", "", "clamp(2.5rem, 6vw, 4.5rem)"},
		{"large", "", "clamp(3.5rem, 10vw, 7rem)"},
		{"xlarge", "", "clamp(4.5rem, 13vw, 9rem)"},
		{"LARGE", "", "clamp(3.5rem, 10vw, 7rem)"},
		{"", "my-fallback", "my-fallback"},
		{"2rem", "", "2rem"},
	}
	for _, c := range cases {
		got := resolveSize(c.size, c.fallback)
		if got != c.want {
			t.Errorf("resolveSize(%q, %q): got %q, want %q", c.size, c.fallback, got, c.want)
		}
	}
}

// ── renderMarkdown ─────────────────────────────────────────────────────────

func TestRenderMarkdown(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello", "<p>hello</p>"},
		{"**bold**", "<strong>bold</strong>"},
		{"[link](https://example.com)", `href="https://example.com"`},
		{"", ""},
	}
	for _, c := range cases {
		got := string(renderMarkdown(c.input))
		if c.want == "" {
			if got != "" {
				t.Errorf("renderMarkdown(%q): got %q, want empty", c.input, got)
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("renderMarkdown(%q): %q not in %q", c.input, c.want, got)
		}
	}
}

// ── isWithinDir ────────────────────────────────────────────────────────────

func TestIsWithinDir(t *testing.T) {
	cases := []struct {
		dir, path string
		want      bool
	}{
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/c", false},
		{"/a/b", "/a/bc", false},
	}
	for _, c := range cases {
		got := isWithinDir(c.dir, c.path)
		if got != c.want {
			t.Errorf("isWithinDir(%q, %q): got %v, want %v", c.dir, c.path, got, c.want)
		}
	}
}

// ── loadRepo ───────────────────────────────────────────────────────────────

func TestLoadRepo(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "myproject")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := `
title:
  text: My Project
  fg: 0xffffff
  size: large
short:
  text: a short description
image: ./img.png
url: https://github.com/example/myproject
theme: dark
description:
  text: ./desc.md
  fg: 0x222222
  bg: 0xf0f0f2
`
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfg), 0o644)
	os.WriteFile(filepath.Join(dir, "img.png"), []byte("PNG"), 0o644)
	os.WriteFile(filepath.Join(dir, "desc.md"), []byte("hello **world**"), 0o644)

	repo, err := loadRepo(base, "myproject")
	if err != nil {
		t.Fatalf("loadRepo: %v", err)
	}
	if repo.TitleText != "My Project" {
		t.Errorf("TitleText: got %q", repo.TitleText)
	}
	if repo.URL != "https://github.com/example/myproject" {
		t.Errorf("URL: got %q", repo.URL)
	}
	if repo.Theme != "dark" {
		t.Errorf("Theme: got %q", repo.Theme)
	}
	if repo.ImagePath != "img/myproject.png" {
		t.Errorf("ImagePath: got %q", repo.ImagePath)
	}
	if !strings.Contains(string(repo.DescHTML), "hello") {
		t.Errorf("DescHTML: %q", repo.DescHTML)
	}
	if !strings.Contains(string(repo.TitleStyle), "clamp(3.5rem") {
		t.Errorf("TitleStyle large keyword not applied: %q", repo.TitleStyle)
	}
}
