package op

import (
	"errors"
	"net/http"

	"github.com/caos/oidc/pkg/oidc"
	"github.com/caos/oidc/pkg/utils"
)

type Introspector interface {
	Decoder() utils.Decoder
	Crypto() Crypto
	Storage() Storage
	AccessTokenVerifier() AccessTokenVerifier
}

type IntrospectorJWTProfile interface {
	Introspector
	JWTProfileVerifier() JWTProfileVerifier
}

func introspectionHandler(introspector Introspector) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		Introspect(w, r, introspector)
	}
}

func Introspect(w http.ResponseWriter, r *http.Request, introspector Introspector) {
	response := oidc.NewIntrospectionResponse()
	token, clientID, err := ParseTokenIntrospectionRequest(r, introspector)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	tokenID, subject, ok := getTokenIDAndSubject(r.Context(), introspector, token)
	if !ok {
		utils.MarshalJSON(w, response)
		return
	}
	err = introspector.Storage().SetIntrospectionFromToken(r.Context(), response, tokenID, subject, clientID)
	if err != nil {
		utils.MarshalJSON(w, response)
		return
	}
	response.SetActive(true)
	utils.MarshalJSON(w, response)
}

func ParseTokenIntrospectionRequest(r *http.Request, introspector Introspector) (token, clientID string, err error) {
	err = r.ParseForm()
	if err != nil {
		return "", "", errors.New("unable to parse request")
	}
	req := new(struct {
		oidc.IntrospectionRequest
		oidc.ClientAssertionParams
	})
	err = introspector.Decoder().Decode(req, r.Form)
	if err != nil {
		return "", "", errors.New("unable to parse request")
	}
	if introspectorJWTProfile, ok := introspector.(IntrospectorJWTProfile); ok && req.ClientAssertion != "" {
		profile, err := VerifyJWTAssertion(r.Context(), req.ClientAssertion, introspectorJWTProfile.JWTProfileVerifier())
		if err == nil {
			return req.Token, profile.Issuer, nil
		}
	}
	clientID, clientSecret, ok := r.BasicAuth()
	if ok {
		if err := introspector.Storage().AuthorizeClientIDSecret(r.Context(), clientID, clientSecret); err != nil {
			return "", "", err
		}
		return req.Token, clientID, nil
	}
	return "", "", errors.New("invalid authorization")
}
