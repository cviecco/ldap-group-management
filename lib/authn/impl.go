package authn

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

type oauth2StateJWT struct {
	Issuer     string   `json:"iss,omitempty"`
	Subject    string   `json:"sub,omitempty"`
	Audience   []string `json:"aud,omitempty"`
	Expiration int64    `json:"exp,omitempty"`
	NotBefore  int64    `json:"nbf,omitempty"`
	IssuedAt   int64    `json:"iat,omitempty"`
	ReturnURL  string   `json:"return_url,omitempty"`
}

type accessToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:expires_in`
	IDToken     string `json:"id_token"`
}

type openidConnectUserInfo struct {
	Subject           string `json:"sub"`
	Name              string `json:"name"`
	Login             string `json:"login,omitempty"`
	Username          string `json:"username,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Email             string `json:"email,omitempty"`
}

type authNCookieJWT struct {
	Issuer     string   `json:"iss,omitempty"`
	Subject    string   `json:"sub,omitempty"`
	Username   string   `json:"username,omitempty"`
	Audience   []string `json:"aud,omitempty"`
	Expiration int64    `json:"exp,omitempty"`
	NotBefore  int64    `json:"nbf,omitempty"`
	IssuedAt   int64    `json:"iat,omitempty"`
}

func randomStringGeneration() (string, error) {
	const size = 32
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

const cookieExpirationHours = 2

func (a *Authenticator) genUserCookieValue(username string, expires time.Time) (string, error) {
	if len(a.sharedSecrets[0]) < 1 {
		return "", errors.New("invalid authenticator state, no shared secrets")
	}
	key := []byte(a.sharedSecrets[0])
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		a.logger.Printf("New jose signer error err: %s", err)
		return "", err
	}
	issuer := a.appName
	subject := "state:" + AuthCookieName
	now := time.Now().Unix()
	stateToken := authNCookieJWT{Issuer: issuer,
		Subject:    subject,
		Username:   username,
		Audience:   []string{issuer},
		NotBefore:  now,
		IssuedAt:   now,
		Expiration: expires.Unix()}
	return jwt.Signed(sig).Claims(stateToken).CompactSerialize()
}

func (s *Authenticator) setAndStoreAuthCookie(w http.ResponseWriter, username string) error {
	expires := time.Now().Add(time.Hour * cookieExpirationHours)
	cookieValue, err := s.genUserCookieValue(username, expires)
	if err != nil {
		return err
	}
	userCookie := http.Cookie{Name: AuthCookieName, Value: cookieValue, Path: "/", Expires: expires, HttpOnly: true, Secure: true}
	http.SetCookie(w, &userCookie)
	return nil
}

func getRedirURL(r *http.Request) string {
	return "https://" + r.Host + Oauth2redirectPath
}

func (s *Authenticator) generateAuthCodeURL(state string, r *http.Request) string {
	var buf bytes.Buffer
	buf.WriteString(s.openID.AuthURL)
	redirectURL := getRedirURL(r)
	v := url.Values{
		"response_type": {"code"},
		"client_id":     {s.openID.ClientID},
		"scope":         {s.openID.Scopes},
		"redirect_uri":  {redirectURL},
	}

	if state != "" {
		// TODO(light): Docs say never to omit state; don't allow empty.
		v.Set("state", state)
	}
	if strings.Contains(s.openID.AuthURL, "?") {
		buf.WriteByte('&')
	} else {
		buf.WriteByte('?')
	}
	buf.WriteString(v.Encode())
	return buf.String()
}

const redirCookieName = "redir_cookie"
const maxAgeSecondsRedirCookie = 300

func (s *Authenticator) generateValidStateString(r *http.Request) (string, error) {
	if len(s.sharedSecrets[0]) < 1 {
		return "", errors.New("invalid authenticator state, no shared secrets")
	}
	key := []byte(s.sharedSecrets[0])
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		log.Printf("New jose signer error err: %s", err)
		return "", err
	}
	issuer := s.appName
	subject := "state:" + redirCookieName
	now := time.Now().Unix()
	stateToken := oauth2StateJWT{Issuer: issuer,
		Subject:    subject,
		Audience:   []string{issuer},
		ReturnURL:  r.URL.String(),
		NotBefore:  now,
		IssuedAt:   now,
		Expiration: now + maxAgeSecondsRedirCookie}
	return jwt.Signed(sig).Claims(stateToken).CompactSerialize()
}

// This is where the redirect to the oath2 provider is computed.
func (s *Authenticator) oauth2DoRedirectoToProviderHandler(w http.ResponseWriter, r *http.Request) {
	stateString, err := s.generateValidStateString(r)
	if err != nil {
		log.Printf("Error from generateValidStateString err: %s\n", err)
		http.Error(w, "Internal Error ", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, s.generateAuthCodeURL(stateString, r), http.StatusFound)
}

// Next are the functions for checking the callback
func (s *Authenticator) JWTClaims(t *jwt.JSONWebToken, dest ...interface{}) (err error) {
	for _, key := range s.sharedSecrets {
		binkey := []byte(key)
		err = t.Claims(binkey, dest...)
		if err == nil {
			return nil
		}
	}
	if err != nil {
		return err
	}
	return errors.New("No valid key found")
}

func getUsernameFromUserinfo(userInfo openidConnectUserInfo) string {
	username := userInfo.Username
	if len(username) < 1 {
		username = userInfo.Login
	}
	if len(username) < 1 {
		username = userInfo.PreferredUsername
	}
	if len(username) < 1 {
		username = userInfo.Email
	}
	return username
}

func (s *Authenticator) getBytesFromSuccessfullPost(url string, data url.Values) ([]byte, error) {
	response, err := s.netClient.PostForm(url, data)
	if err != nil {
		//s.logger.Debugf(1, "client post error err: %s\n", err)
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		//s.logger.Debugf(1, "Error reading http responseBody err: %s\n", err)
		return nil, err
	}

	if response.StatusCode >= 300 {
		//s.logger.Debugf(1, string(responseBody))
		return nil, errors.New("invalid status code")
	}
	return responseBody, nil
}

func (s *Authenticator) getVerifyReturnStateJWT(r *http.Request) (oauth2StateJWT, error) {
	inboundJWT := oauth2StateJWT{}
	serializedState := r.URL.Query().Get("state")
	if len(serializedState) < 1 {
		return inboundJWT, errors.New("null inbound state")
	}
	tok, err := jwt.ParseSigned(serializedState)
	if err != nil {
		return inboundJWT, err
	}
	if err := s.JWTClaims(tok, &inboundJWT); err != nil {
		//s.logger.Debugf(1, "error parsing claims err: %s\n", err)
		return inboundJWT, err
	}
	// At this point we know the signature is valid, but now we must
	// validate the contents of the JWT token
	issuer := s.appName
	subject := "state:" + redirCookieName
	if inboundJWT.Issuer != issuer || inboundJWT.Subject != subject ||
		inboundJWT.NotBefore > time.Now().Unix() || inboundJWT.Expiration < time.Now().Unix() {
		err = errors.New("invalid JWT values")
		return inboundJWT, err
	}
	return inboundJWT, nil
}

func (s *Authenticator) oauth2RedirectPathHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		//s.logger.Printf("Bad method on redirect, should only be GET")
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	authCode := r.URL.Query().Get("code")
	if len(authCode) < 1 {
		s.logger.Println("null code")
		http.Error(w, "null code", http.StatusUnauthorized)
		return
	}
	inboundJWT, err := s.getVerifyReturnStateJWT(r)
	if err != nil {
		s.logger.Printf("error processing state err: %s\n", err)
		http.Error(w, "null or bad inboundState", http.StatusUnauthorized)
		return
	}
	// OK state  is valid.. now we perform the token exchange
	redirectURL := getRedirURL(r)
	tokenRespBody, err := s.getBytesFromSuccessfullPost(s.openID.TokenURL,
		url.Values{"redirect_uri": {redirectURL},
			"code":          {authCode},
			"grant_type":    {"authorization_code"},
			"client_id":     {s.openID.ClientID},
			"client_secret": {s.openID.ClientSecret},
		})
	if err != nil {
		s.logger.Printf("Error getting byes fom post err: %s", err)
		http.Error(w, "bad transaction with openic context ", http.StatusInternalServerError)
		return
	}
	var oauth2AccessToken accessToken
	err = json.Unmarshal(tokenRespBody, &oauth2AccessToken)
	if err != nil {
		s.logger.Printf(string(tokenRespBody))
		http.Error(w, "cannot decode oath2 response for token ", http.StatusInternalServerError)
		return
	}
	// TODO: tolower
	if oauth2AccessToken.TokenType != "Bearer" || len(oauth2AccessToken.AccessToken) < 1 {
		s.logger.Printf("token type invalid token=%s", string(tokenRespBody))
		http.Error(w, "invalid accessToken ", http.StatusInternalServerError)
		return
	}

	// Now we use the access_token (from token exchange) to get userinfo
	userInfoRespBody, err := s.getBytesFromSuccessfullPost(s.openID.UserinfoURL,
		url.Values{"access_token": {oauth2AccessToken.AccessToken}})
	if err != nil {
		s.logger.Println(err)
		http.Error(w, "bad transaction with openic context ", http.StatusInternalServerError)
		return
	}
	var userInfo openidConnectUserInfo
	err = json.Unmarshal(userInfoRespBody, &userInfo)
	if err != nil {
		s.logger.Printf("Error unmarshalling userinfo ")
		//s.logger.Debugf(1, "unmarshal error %s\n", string(tokenRespBody))
		http.Error(w, "cannot decode oath2 userinfo token ", http.StatusInternalServerError)
		return
	}
	username := getUsernameFromUserinfo(userInfo)

	err = s.setAndStoreAuthCookie(w, username)
	if err != nil {
		s.logger.Println(err)
		http.Error(w, "cannot set auth Cookie", http.StatusInternalServerError)
		return
	}

	destinationPath := inboundJWT.ReturnURL
	http.Redirect(w, r, destinationPath, http.StatusFound)
}

// validateUserCookieValue returns "" if no or bad username, returns non-nil error for fatal errors only
func (s *Authenticator) validateUserCookieValue(remoteCookieValue string) (string, error) {
	inboundJWT := authNCookieJWT{}
	if len(remoteCookieValue) < 1 {
		s.logger.Printf("Invalid cookie value (too small)")
		return "", nil
	}
	tok, err := jwt.ParseSigned(remoteCookieValue)
	if err != nil {
		s.logger.Printf("Invalid cookie value(jwt) (%s)", err)
		return "", nil
	}
	if err := s.JWTClaims(tok, &inboundJWT); err != nil {
		s.logger.Printf("error validating JWT claims err: %s\n", err)
		// TODO: this path could have fatal errors, need to take this into account
		// to avoid a potential redirect loop.
		return "", nil
	}
	// At this point we know the signature is valid, but now we must
	// validate the contents of the JWT token
	issuer := s.appName
	subject := "state:" + AuthCookieName
	if inboundJWT.Issuer != issuer || inboundJWT.Subject != subject ||
		inboundJWT.NotBefore > time.Now().Unix() || inboundJWT.Expiration < time.Now().Unix() {
		s.logger.Printf("invalid JWT values")
		return "", nil
	}
	username := inboundJWT.Username
	if len(username) < 1 {
		return "", errors.New("bad cookie Vauue state")
	}
	return inboundJWT.Username, nil

}

func (s *Authenticator) getRemoteUserName(w http.ResponseWriter, r *http.Request) (string, error) {
	// If you have a verified cert, no need for cookies
	if r.TLS != nil {
		if len(r.TLS.VerifiedChains) > 0 {
			clientName := r.TLS.VerifiedChains[0][0].Subject.CommonName
			return clientName, nil
		}
	}

	if s.setHeadersFunc != nil {
		err := s.setHeadersFunc(w)
		if err != nil {
			return "", err
		}
	}

	remoteCookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		//s.logger.Debugf(1, "Err cookie %s", err)
		s.oauth2DoRedirectoToProviderHandler(w, r)
		return "", err
	}
	username, err := s.validateUserCookieValue(remoteCookie.Value)
	if err != nil {
		http.Error(w, "bad transaction with openic context ", http.StatusInternalServerError)
		return "", err
	}
	if username == "" {
		log.Printf("invalid Cookie Value")
		s.oauth2DoRedirectoToProviderHandler(w, r)
		return "", errors.New("Invalid Cookie Value")

	}
	return username, nil
}
