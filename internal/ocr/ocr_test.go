package ocr

import (
	"reflect"
	"testing"
)

func TestCandidates(t *testing.T) {
	got := candidates("Kode: abc-123\nABC-123\nAB")
	want := []string{"ABC-123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates() = %#v, want %#v", got, want)
	}
}

func TestReadRejectsUnsupportedImageBeforeRunningEngine(t *testing.T) {
	reader := New("does-not-exist", "ind+eng")
	if _, err := reader.Read(t.Context(), []byte("not an image")); err != ErrUnsupportedImage {
		t.Fatalf("Read() error = %v, want %v", err, ErrUnsupportedImage)
	}
}
