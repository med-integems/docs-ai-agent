package services

type googleService struct {
	apiKey string
}

func NewGoogleService(apiKey string) *googleService {
	return &googleService{apiKey: apiKey}
}
