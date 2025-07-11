package auth

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	fbAuth "firebase.google.com/go/auth"
	"github.com/visionex-project/visionex/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirebaseAuthClient interface {
	VerifyIDToken(ctx context.Context, idToken string) (*fbAuth.Token, error)
}

type Authenticator struct {
	client FirebaseAuthClient
}

func New(client FirebaseAuthClient) *Authenticator {
	return &Authenticator{
		client: client,
	}
}

func (a *Authenticator) Verify(ctx context.Context, token string) (string, error) {
	decodedToken, err := a.client.VerifyIDToken(ctx, token)
	if err != nil {
		return "", status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}
	rawEmail, ok := decodedToken.Claims["email"]
	if !ok {
		return "", fmt.Errorf("failed to verify the token: invalid email in claim")
	}

	email, ok := rawEmail.(string)
	if !ok {
		return "", fmt.Errorf("failed to verify the token: invalid email in claim")
	}

	_, err = mail.ParseAddress(email)
	if err != nil {
		return "", fmt.Errorf("failed to verify the token: invalid email format")
	}
	splitEmail := strings.Split(email, "@")
	if len(splitEmail) != 2 {
		return "", fmt.Errorf("failed to verify the token: malformed email structure (expected single '@')")
	}
	domain := splitEmail[1]
	if !utils.Contains(validEmailDomains, domain) {
		return "", fmt.Errorf("failed to verify the token: invalid email domain")
	}

	return token, nil
}
