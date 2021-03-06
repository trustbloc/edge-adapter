/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package bddutil

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	urllib "net/url"
	"reflect"
	"strings"
	"time"

	docdid "github.com/hyperledger/aries-framework-go/pkg/doc/did"
	vdriapi "github.com/hyperledger/aries-framework-go/pkg/framework/aries/api/vdr"
	"github.com/trustbloc/edge-core/pkg/log"
)

var logger = log.New("hub-router/bddutil")

func checkRedirect(req *http.Request, via []*http.Request) error {
	logger.Infof("Redirect: method=%s url=%s", req.Method, req.URL.String())

	CookieData(req.URL)

	return nil
}

type suffixList struct{}

func (sl *suffixList) PublicSuffix(string) string {
	return ""
}

func (sl *suffixList) String() string {
	return "nil suffixlist - allows all cookie sharing"
}

// nolint:gochecknoglobals
var sharedJar *cookiejar.Jar

// HTTPDo util to send http requests.
func HTTPDo(method, url, contentType, token string, body io.Reader, tlsConfig *tls.Config) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Add("Content-Type", contentType)
	}

	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	if sharedJar == nil {
		sharedJar, err = cookiejar.New(&cookiejar.Options{PublicSuffixList: &suffixList{}})
		if err != nil {
			return nil, err
		}
	}

	httpClient := &http.Client{
		Jar: sharedJar,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		CheckRedirect: checkRedirect,
	}

	return httpClient.Do(req)
}

// CookieData logs the cookies contained within the bdd shared cookie jar for the given url.
func CookieData(url *urllib.URL) {
	if sharedJar == nil {
		return
	}

	cookies := sharedJar.Cookies(url)

	logger.Infof("cookies for url: %s", url.String())

	for _, cookie := range cookies {
		logger.Infof("  %#v", cookie)
	}
}

// ExpectedStatusCodeError formats the status code error message.
func ExpectedStatusCodeError(expected, actual int, respBytes []byte) error {
	return fmt.Errorf("expected status code %d but got status code %d with response body %s instead",
		expected, actual, respBytes)
}

// CloseResponseBody closes the response body.
func CloseResponseBody(respBody io.Closer) {
	err := respBody.Close()
	if err != nil {
		logger.Errorf("Failed to close response body: %s", err.Error())
	}
}

// ResolveDID waits for the DID to become available for resolution.
func ResolveDID(vdriRegistry vdriapi.Registry, did string, maxRetry int) (*docdid.Doc, error) {
	var docResolution *docdid.DocResolution

	for i := 1; i <= maxRetry; i++ {
		var err error
		docResolution, err = vdriRegistry.Resolve(did)

		if err != nil {
			if !strings.Contains(err.Error(), "DID does not exist") {
				return nil, err
			}

			fmt.Printf("did %s not found - will retry %d of %d\n", did, i, maxRetry)
			time.Sleep(3 * time.Second) // nolint:gomnd

			continue
		}
	}

	return docResolution.DIDDocument, nil
}

// GetDIDConnectRequestKey key for storing DID Connect request.
func GetDIDConnectRequestKey(issuerID, agentID string) string {
	return issuerID + agentID + "-didconnect-request"
}

// GetDIDConnectResponseKey key for storing DID Connect response.
func GetDIDConnectResponseKey(issuerID, agentID string) string {
	return issuerID + agentID + "-didconnect-response"
}

// GetDeepLinkWalletInvitationKey key for wallet bridge invitation URLs.
func GetDeepLinkWalletInvitationKey(userID string) string {
	return userID + "-walletbridge-invitation"
}

// GetRemoteWalletAppInfo key for wallet info of remote wallet app.
func GetRemoteWalletAppInfo(userID string) string {
	return userID + "-remote-wallet-appinfo"
}

// JSONMarshaller can marshal itself to JSON bytes.
type JSONMarshaller interface {
	MarshalJSON() ([]byte, error)
}

// DecodeJSONMarshaller decodes the JSONMarshaller into the given object.
func DecodeJSONMarshaller(jm JSONMarshaller, custom interface{}) error {
	bits, err := jm.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to execute MarshalJSON() : %w", err)
	}

	err = json.NewDecoder(bytes.NewReader(bits)).Decode(custom)
	if err != nil {
		return fmt.Errorf("failed to decode custom jsonmarshaller : %w", err)
	}

	return nil
}

// StringsContains check if the string is present in the string array.
func StringsContains(val string, slice []string) bool {
	for _, s := range slice {
		if val == s {
			return true
		}
	}

	return false
}

// SendHTTP util to send http requests.
func SendHTTP(method, destination string, message []byte, result interface{}) error {
	// create request
	req, err := http.NewRequest(method, destination, bytes.NewBuffer(message))
	if err != nil {
		return fmt.Errorf("failed to create new http '%s' request for '%s', cause: %s", method, destination, err)
	}

	// set headers
	req.Header.Set("Content-Type", "application/json")

	// send http request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get response from '%s', cause :%s", destination, err)
	}

	defer CloseResponseBody(resp.Body)

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response from '%s', cause :%s", destination, err)
	}

	logger.Debugf("Got response from '%s' [method: %s], response payload: %s", destination, method, string(data))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get successful response from '%s', unexpected status code [%d], "+
			"and message [%s]", destination, resp.StatusCode, string(data))
	}

	if result == nil {
		return nil
	}

	return json.Unmarshal(data, result)
}

// JSONBytesEqual compares 2 JSON bytes
func JSONBytesEqual(a, b []byte) (bool, error) {
	var ar, br interface{}

	if err := json.Unmarshal(a, &ar); err != nil {
		return false, err
	}

	if err := json.Unmarshal(b, &br); err != nil {
		return false, err
	}

	return reflect.DeepEqual(br, ar), nil
}
