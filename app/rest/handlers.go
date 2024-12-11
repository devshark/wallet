package rest

import (
	"log"

	"github.com/devshark/wallet/app/internal/repository"
)

type RestHandlers struct {
	repo   repository.Repository
	logger *log.Logger
}

func NewRestHandlers(repo repository.Repository) *RestHandlers {
	return &RestHandlers{
		repo:   repo,
		logger: log.Default(),
	}
}
