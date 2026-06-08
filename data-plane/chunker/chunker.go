package chunker

const (
	DefaultChunkSize = 512
	DefaultOverlap   = 64
)

type Options struct {
	ChunkSize int
	Overlap   int
}

func (o *Options) applyDefaults() {
	if o.ChunkSize == 0 {
		o.ChunkSize = DefaultChunkSize
		if o.Overlap == 0 {
			o.Overlap = DefaultOverlap
		}
	}
}

type Chunk struct {
	Text  string
	Index int
}

func Split(text string, opts Options) []Chunk {
	opts.applyDefaults()

	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	var chunks []Chunk
	start := 0
	idx := 0

	for start < len(runes) {
		end := start + opts.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunks = append(chunks, Chunk{
			Text:  string(runes[start:end]),
			Index: idx,
		})
		idx++

		next := start + opts.ChunkSize - opts.Overlap
		if next <= start {
			next = start + 1
		}
		start = next
	}

	return chunks
}
