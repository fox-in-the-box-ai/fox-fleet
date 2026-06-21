package spa

import (
	"strings"
	"testing"
)

func TestIndexHTML_HasFaviconLinks(t *testing.T) {
	data, err := Static.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(data)

	checks := []string{
		`rel="icon" type="image/svg+xml" href="favicon.svg"`,
		`rel="icon" type="image/png" sizes="32x32" href="favicon-32x32.png"`,
		`rel="icon" type="image/png" sizes="16x16" href="favicon-16x16.png"`,
		`rel="apple-touch-icon" sizes="180x180" href="apple-touch-icon.png"`,
		`rel="shortcut icon" href="favicon.ico"`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("index.html missing favicon link: %s", want)
		}
	}
}

func TestFaviconFiles_Embedded(t *testing.T) {
	files := []string{
		"static/favicon.svg",
		"static/favicon.ico",
		"static/favicon-16x16.png",
		"static/favicon-32x32.png",
		"static/apple-touch-icon.png",
	}
	for _, f := range files {
		data, err := Static.ReadFile(f)
		if err != nil {
			t.Errorf("missing embedded file %s: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("embedded file %s is empty", f)
		}
	}
}
