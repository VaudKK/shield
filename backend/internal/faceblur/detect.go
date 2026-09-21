// Package faceblur detects faces in an image so a disclosure package can
// cover them before the image leaves Shield. Detection is a risk signal
// like content-safety classification: a missed face means nothing was
// found to blur, never a reason to fail or block an export, and callers
// should always encourage a manual look before sharing.
package faceblur

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/jpeg" // decoder registration for image.Decode
	_ "image/png"  // decoder registration for image.Decode
	"sync"

	pigo "github.com/esimov/pigo/core"

	_ "golang.org/x/image/webp" // decoder registration for image.Decode
)

//go:embed cascade/facefinder
var cascadeFS embed.FS

var (
	classifierOnce sync.Once
	classifier     *pigo.Pigo
	classifierErr  error
)

func loadClassifier() (*pigo.Pigo, error) {
	classifierOnce.Do(func() {
		data, err := cascadeFS.ReadFile("cascade/facefinder")
		if err != nil {
			classifierErr = fmt.Errorf("read face cascade: %w", err)
			return
		}
		classifier, classifierErr = pigo.NewPigo().Unpack(data)
		if classifierErr != nil {
			classifierErr = fmt.Errorf("unpack face cascade: %w", classifierErr)
		}
	})
	return classifier, classifierErr
}

// minScore is the detection quality threshold below which a candidate is
// discarded as noise rather than treated as an actual face.
const minScore = 5.0

// padMultiplier expands each detected (square) face region beyond pigo's
// raw box so a drawn box covers hairline and chin, not just a tight crop
// of facial features — e.g. 1.3 means the drawn box is 30% larger than the
// raw detected square, not 30% larger per side.
const padMultiplier = 1.3

// Detect returns a bounding box for every face found in data (decoded as
// an image). A nil, non-error result means no faces were found — that is
// a normal outcome, not a failure, and callers should say so explicitly
// rather than silently treating it as "nothing to protect."
func Detect(data []byte) ([]image.Rectangle, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	cl, err := loadClassifier()
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	cols, rows := bounds.Dx(), bounds.Dy()
	pixels := pigo.RgbToGrayscale(img)

	cParams := pigo.CascadeParams{
		MinSize:     20,
		MaxSize:     1000,
		ShiftFactor: 0.1,
		ScaleFactor: 1.1,
		ImageParams: pigo.ImageParams{
			Pixels: pixels,
			Rows:   rows,
			Cols:   cols,
			Dim:    cols,
		},
	}

	dets := cl.RunCascade(cParams, 0.0)
	dets = cl.ClusterDetections(dets, 0.2)

	var regions []image.Rectangle
	for _, d := range dets {
		if d.Q < minScore {
			continue
		}
		half := int(float64(d.Scale) * padMultiplier / 2)
		r := image.Rect(d.Col-half, d.Row-half, d.Col+half, d.Row+half).Intersect(bounds)
		if !r.Empty() {
			regions = append(regions, r)
		}
	}

	return regions, nil
}
