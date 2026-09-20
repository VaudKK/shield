// Package evidence orchestrates evidence ingestion: file-type validation,
// hashing, storage, metadata persistence, and audit logging. It has no
// knowledge of HTTP.
package evidence

import (
	"fmt"
	"mime/multipart"
	"net/http"
)

// allowedMimeTypes are the only file types Shield accepts for the MVP, as
// sniffed from file content — never trusted from the client-supplied
// Content-Type header.
var allowedMimeTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
}

const sniffBufferSize = 512

// ErrUnsupportedFileType is returned when the file's sniffed content type
// is not in the supported set.
var ErrUnsupportedFileType = fmt.Errorf("unsupported file type; only JPEG, PNG, WEBP, and PDF are supported")

// sniffAndValidate reads a small prefix of file to determine its real
// content type via magic bytes, then resets the read position so the full
// file can still be streamed from the start. It returns the sniffed MIME
// type and its canonical extension.
func sniffAndValidate(file multipart.File) (mimeType, ext string, err error) {
	buf := make([]byte, sniffBufferSize)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return "", "", fmt.Errorf("read file for sniffing: %w", err)
	}

	sniffed := http.DetectContentType(buf[:n])

	if _, err := file.Seek(0, 0); err != nil {
		return "", "", fmt.Errorf("reset file position after sniffing: %w", err)
	}

	ext, ok := allowedMimeTypes[sniffed]
	if !ok {
		return "", "", ErrUnsupportedFileType
	}

	return sniffed, ext, nil
}
