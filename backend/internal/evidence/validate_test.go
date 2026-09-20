package evidence

import (
	"bytes"
	"io"
	"mime/multipart"
	"testing"
)

// memFile adapts a byte slice to multipart.File (Reader + ReaderAt + Seeker + Closer).
type memFile struct {
	*bytes.Reader
}

func (memFile) Close() error { return nil }

func newMemFile(b []byte) multipart.File {
	return memFile{bytes.NewReader(b)}
}

func TestSniffAndValidate_AllowedTypes(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		wantMIME string
	}{
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 'J', 'F', 'I', 'F'}, "image/jpeg"},
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, "image/png"},
		{"pdf", []byte("%PDF-1.4\n%rest of file"), "application/pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMemFile(tt.content)
			mime, _, err := sniffAndValidate(f)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mime != tt.wantMIME {
				t.Errorf("expected mime %q, got %q", tt.wantMIME, mime)
			}

			// Position must be reset so the caller can re-read from the start.
			rest, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("read after sniff: %v", err)
			}
			if !bytes.Equal(rest, tt.content) {
				t.Error("expected file position to be reset to the start after sniffing")
			}
		})
	}
}

func TestSniffAndValidate_RejectsUnsupportedType(t *testing.T) {
	// A plain text file / executable-like header should be rejected even
	// though it has a benign-looking size and no declared Content-Type.
	f := newMemFile([]byte("#!/bin/sh\necho hello\n"))

	_, _, err := sniffAndValidate(f)
	if err == nil {
		t.Fatal("expected an error for an unsupported file type")
	}
}

func TestSniffAndValidate_IgnoresClientClaimedType(t *testing.T) {
	// Even if a client were to claim this is a PDF via Content-Type, the
	// sniffed content type must be what's actually validated. This is a
	// plain script disguised with a .pdf-like name; sniffing must still
	// reject it since the magic bytes don't match any allowed type.
	f := newMemFile([]byte("<script>alert(1)</script>"))

	_, _, err := sniffAndValidate(f)
	if err == nil {
		t.Fatal("expected sniffing to reject content based on magic bytes, not filename or claimed type")
	}
}
