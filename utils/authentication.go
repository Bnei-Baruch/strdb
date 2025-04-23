package utils

import (
	"context"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"net/http"
	"strings"
	"time"
)

type Roles struct {
	Roles []string `json:"roles"`
}

type IDTokenClaims struct {
	Acr               string           `json:"acr"`
	AllowedOrigins    []string         `json:"allowed-origins"`
	Aud               interface{}      `json:"aud"`
	AuthTime          int              `json:"auth_time"`
	Azp               string           `json:"azp"`
	Email             string           `json:"email"`
	Exp               int              `json:"exp"`
	FamilyName        string           `json:"family_name"`
	GivenName         string           `json:"given_name"`
	Iat               int              `json:"iat"`
	Iss               string           `json:"iss"`
	Jti               string           `json:"jti"`
	Name              string           `json:"name"`
	Nbf               int              `json:"nbf"`
	Nonce             string           `json:"nonce"`
	PreferredUsername string           `json:"preferred_username"`
	RealmAccess       Roles            `json:"realm_access"`
	ResourceAccess    map[string]Roles `json:"resource_access"`
	SessionState      string           `json:"session_state"`
	Sub               string           `json:"sub"`
	Typ               string           `json:"typ"`
}

type User struct {
	ID        int64     `boil:"id" json:"id" toml:"id" yaml:"id"`
	Email     string    `boil:"email" json:"email" toml:"email" yaml:"email"`
	Name      string    `boil:"name" json:"name,omitempty" toml:"name" yaml:"name,omitempty"`
	CreatedAt time.Time `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	AccountID string    `boil:"account_id" json:"account_id" toml:"account_id" yaml:"account_id"`
}

func AuthenticationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		if viper.GetBool("authentication.enable") {
			tokenVerifier := c.MustGet("TOKEN_VERIFIER").(*oidc.IDTokenVerifier)

			header, err := parseToken(c)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err).SetType(gin.ErrorTypePublic)
				return
			}

			token, err := tokenVerifier.Verify(context.TODO(), header)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err).SetType(gin.ErrorTypePublic)
				return
			}
			c.Set("ID_TOKEN", token)

			var claims IDTokenClaims
			if err = token.Claims(&claims); err != nil {
				c.AbortWithError(http.StatusBadRequest, err).SetType(gin.ErrorTypePublic)
				return
			}
			c.Set("ID_TOKEN_CLAIMS", claims)

			user, err := getUser(&claims)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err).SetType(gin.ErrorTypePrivate)
				return
			}
			c.Set("USER", user)
		}

		c.Next()
	}
}

func getUser(claims *IDTokenClaims) (*User, error) {
	user := &User{
		AccountID: claims.Sub,
		Email:     claims.Email,
		Name:      fmt.Sprintf("%s %s", claims.GivenName, claims.FamilyName),
	}
	return user, nil
}

func parseToken(c *gin.Context) (string, error) {
	authHeader := strings.Split(strings.TrimSpace(c.Request.Header.Get("Authorization")), " ")
	if len(authHeader) == 2 && strings.ToLower(authHeader[0]) == "bearer" && len(authHeader[1]) > 0 {
		return authHeader[1], nil
	}
	return "", errors.New("no `Authorization` header set")
}
