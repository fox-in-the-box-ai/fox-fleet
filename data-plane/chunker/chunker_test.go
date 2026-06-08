package chunker

import (
	"strings"
	"testing"
)

func TestSplit_Empty(t *testing.T) {
	chunks := Split("", Options{})
	if len(chunks) != 0 {
		t.Errorf("len(chunks) = %d, want 0", len(chunks))
	}
}

func TestSplit_ShortText(t *testing.T) {
	chunks := Split("hello world", Options{ChunkSize: 100, Overlap: 10})
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}
	if chunks[0].Text != "hello world" {
		t.Errorf("Text = %q", chunks[0].Text)
	}
	if chunks[0].Index != 0 {
		t.Errorf("Index = %d", chunks[0].Index)
	}
}

func TestSplit_Overlap(t *testing.T) {
	text := strings.Repeat("a", 20)
	chunks := Split(text, Options{ChunkSize: 10, Overlap: 3})

	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want >= 2", len(chunks))
	}

	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk[%d].Index = %d", i, c.Index)
		}
	}

	if len(chunks[0].Text) != 10 {
		t.Errorf("first chunk len = %d, want 10", len(chunks[0].Text))
	}
}

func TestSplit_Defaults(t *testing.T) {
	text := strings.Repeat("x", 1000)
	chunks := Split(text, Options{})

	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want >= 2 (1000 chars with default 512 chunk)", len(chunks))
	}

	if len([]rune(chunks[0].Text)) != DefaultChunkSize {
		t.Errorf("first chunk size = %d, want %d", len([]rune(chunks[0].Text)), DefaultChunkSize)
	}
}

func TestSplit_ExactSize(t *testing.T) {
	text := strings.Repeat("b", 10)
	chunks := Split(text, Options{ChunkSize: 10, Overlap: 0})

	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}
	if chunks[0].Text != text {
		t.Errorf("Text = %q", chunks[0].Text)
	}
}

func TestSplit_Unicode(t *testing.T) {
	text := strings.Repeat("日本語", 10) // 30 runes
	chunks := Split(text, Options{ChunkSize: 10, Overlap: 2})

	if len(chunks) < 3 {
		t.Fatalf("len(chunks) = %d, want >= 3", len(chunks))
	}

	totalRunes := 0
	seen := map[int]bool{}
	for _, c := range chunks {
		r := []rune(c.Text)
		totalRunes += len(r)
		seen[c.Index] = true
	}
	if totalRunes <= 30 {
		t.Error("total rune count should exceed input due to overlap")
	}
}

func TestSplit_ConsecutiveIndices(t *testing.T) {
	text := strings.Repeat("c", 100)
	chunks := Split(text, Options{ChunkSize: 20, Overlap: 5})

	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk[%d].Index = %d, want %d", i, c.Index, i)
		}
	}
}
