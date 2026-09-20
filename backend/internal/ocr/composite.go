package ocr

import "context"

// CompositeService dispatches to the first backend that reports the file
// type as Applicable.
type CompositeService struct {
	backends []Service
}

func NewCompositeService(backends ...Service) *CompositeService {
	return &CompositeService{backends: backends}
}

func (s *CompositeService) ExtractText(ctx context.Context, data []byte, mimeType string) (Result, error) {
	for _, backend := range s.backends {
		result, err := backend.ExtractText(ctx, data, mimeType)
		if err != nil {
			return Result{}, err
		}
		if result.Applicable {
			return result, nil
		}
	}
	return Result{Applicable: false}, nil
}

// ExtractWordBoxes dispatches to the first backend that both implements
// WordBoxService and returns a non-empty result for mimeType. Returns nil,
// nil (not an error) when no backend supports word boxes for this type —
// callers should treat that the same as "pixel redaction unavailable for
// this file type", not a failure.
func (s *CompositeService) ExtractWordBoxes(ctx context.Context, data []byte, mimeType string) ([]BoxedWord, error) {
	for _, backend := range s.backends {
		boxer, ok := backend.(WordBoxService)
		if !ok {
			continue
		}
		boxes, err := boxer.ExtractWordBoxes(ctx, data, mimeType)
		if err != nil {
			return nil, err
		}
		if boxes != nil {
			return boxes, nil
		}
	}
	return nil, nil
}
