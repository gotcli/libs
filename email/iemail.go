package email

import (
	"github.com/d-vignesh/go-jwt-auth/utils"
	"github.com/hashicorp/go-hclog"
)

type SGMailService struct {
	logger  hclog.Logger
	configs *utils.Configurations
}
