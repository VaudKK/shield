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
