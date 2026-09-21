package faceblur

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestDetect_FindsFaceInKnownSample(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_face.jpg")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	regions, err := Detect(data)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("expected at least one face to be detected in the sample image")
	}
	for _, r := range regions {
		if r.Empty() {
			t.Errorf("got an empty region: %v", r)
		}
	}
}

func TestDetect_RegionIsNotTheWholeImage(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_face.jpg")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	regions, err := Detect(data)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("expected at least one face to be detected")
	}

	imgArea := img.Bounds().Dx() * img.Bounds().Dy()
	for _, r := range regions {
		regionArea := r.Dx() * r.Dy()
		// A padded face box should comfortably cover a face, not balloon
		// out (via clipping at the image edges) to the entire frame.
		if float64(regionArea) > 0.95*float64(imgArea) {
			t.Errorf("region %v covers %.0f%% of the image — padding is too aggressive", r, 100*float64(regionArea)/float64(imgArea))
		}
	}
}

func TestDetect_NoFacesInPlainImage(t *testing.T) {
	// A solid-color image has no facial features at all.
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := range 200 {
		for x := range 200 {
			img.Set(x, y, color.RGBA{120, 140, 160, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	regions, err := Detect(buf.Bytes())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(regions) != 0 {
		t.Errorf("expected no faces in a plain color image, got %d", len(regions))
	}
}

func TestDetect_RejectsUndecodableData(t *testing.T) {
	_, err := Detect([]byte("not an image"))
	if err == nil {
		t.Fatal("expected an error for undecodable data")
	}
}
