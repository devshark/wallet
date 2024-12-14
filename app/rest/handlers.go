package rest

import (
	"log"

	"github.com/devshark/wallet/app/internal/repository"
)

type Handlers struct {
	repo    repository.Repository
	logger  *log.Logger
	pingers []Pinger
}

func NewRestHandlers(repo repository.Repository) *Handlers {
	return &Handlers{
		repo:   repo,
		logger: log.Default(),
	}
}
