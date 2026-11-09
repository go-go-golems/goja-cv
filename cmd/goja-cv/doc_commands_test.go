package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/doc"
)

// shellWords splits a command line like a POSIX shell for the cases the docs
// use: single and double quotes, backslash escapes outside single quotes.
// It stops at an unquoted |, <, >, ; or #comment.
func shellWords(s string) []string {
	var out []string
	var cur strings.Builder
	inWord, sq, dq := false, false, false
	flush := func() {
		if inWord {
			out = append(out, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case sq:
			if c == '\'' {
				sq = false
			} else {
				cur.WriteByte(c)
			}
		case dq:
			switch {
			case c == '"':
				dq = false
			case c == '\\' && i+1 < len(s):
				i++
				cur.WriteByte(s[i])
			default:
				cur.WriteByte(c)
			}
		case c == '\'':
			sq, inWord = true, true
		case c == '"':
			dq, inWord = true, true
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			inWord = true
		case c == ' ' || c == '\t':
			flush()
		case (c == '|' || c == '<' || c == '>' || c == ';') || (c == '#' && !inWord):
			flush()
			return out
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	flush()
	return out
}

var (
	fenceRe  = regexp.MustCompile("(?s)```([a-z]*)[^\\n]*\\n(.*?)```")
	inlineRe = regexp.MustCompile("`(goja-cv [^`]+)`")
	cmdRe    = regexp.MustCompile(`(?:^|[\s(])(goja-cv [a-z].*)$`)
)

// docCommands extracts the goja-cv command lines of a Markdown document:
// lines of shell code blocks (with \ continuations joined) and inline code.
func docCommands(md string) []string {
	var out []string
	for _, m := range fenceRe.FindAllStringSubmatch(md, -1) {
		switch m[1] {
		case "bash", "sh", "shell", "console", "":
		default:
			continue
		}
		body := strings.ReplaceAll(m[2], "\\\n", " ")
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimPrefix(strings.TrimSpace(line), "$ ")
			if c := cmdRe.FindStringSubmatch(line); c != nil {
				out = append(out, c[1])
			}
		}
	}
	for _, m := range inlineRe.FindAllStringSubmatch(md, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestDocumentedCommandsParse: every goja-cv command shown in the guidebook
// and the README parses (subcommand and flags) — run with --help appended.
func TestDocumentedCommandsParse(t *testing.T) {
	pages, err := doc.Pages()
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, p := range pages {
		sources[p.File] = p.Body
	}
	if readme, err := os.ReadFile(filepath.Join("..", "..", "README.md")); err == nil {
		sources["README.md"] = string(readme)
	}
	n := 0
	for name, md := range sources {
		for _, line := range docCommands(md) {
			args := shellWords(line)
			if len(args) < 2 || args[0] != "goja-cv" {
				continue
			}
			args = append(args[1:], "--help")
			n++
			code, out, errOut := runCLI(t, args...)
			if code != exitOK {
				t.Fatalf("%s: %q exits %d with --help:\n%s\n%s", name, line, code, out, errOut)
			}
		}
	}
	t.Logf("%d documented commands parse", n)
	if n < 25 {
		t.Fatalf("only %d documented commands found", n)
	}
}

func TestShellWords(t *testing.T) {
	got := shellWords(`goja-cv eval -e 'return { a: "x" }' --image "my scan.tif" a\ b # comment`)
	want := []string{"goja-cv", "eval", "-e", `return { a: "x" }`, "--image", "my scan.tif", "a b"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
}
