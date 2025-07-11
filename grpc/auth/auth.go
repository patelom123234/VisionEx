package auth

import (
	"context"
)

type Auth interface {
	Verify(ctx context.Context, token string) (string, error)
}

var validEmailDomains = []string{
	"yanolja.com",
	"ezeetechnosys.com",
	"yanoljacloudsolution.com",
	"interparktriple.com",
	"nol-universe.com",
	"goglobal.travel",
	"group.yanolja.com",
}
