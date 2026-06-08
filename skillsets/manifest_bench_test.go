package skillsets

import "testing"

var benchManifestYAML = []byte(validManifestYAML)

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(benchManifestYAML); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidate(b *testing.B) {
	m, err := Parse(benchManifestYAML)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := Validate(m); err != nil {
			b.Fatal(err)
		}
	}
}
