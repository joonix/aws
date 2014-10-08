package aws

import (
	"errors"
	"github.com/smartystreets/go-aws-auth"
	"io/ioutil"
	"net/http"
	"net/url"
)

// DefaultSigner provides a working Signer using the smartystreets awsauth library.
var DefaultSigner = SignerFunc(func(r *http.Request) {
	awsauth.Sign(r)
})

// Signer describes how to Sign requests before sending them to Amazon Web Services API.
type Signer interface {
	Sign(*http.Request)
}

// SignerFunc wraps a function to implement the Signer interface.
type SignerFunc func(*http.Request)

func (f SignerFunc) Sign(r *http.Request) {
	f(r)
}

type TagItem struct {
	Key   string `xml:"key"`
	Value string `xml:"value"`
}

// SignedRequester handles talking with the Amazon API and signing of our requests.
type SignedRequester interface {
	SignedRequest(v url.Values) ([]byte, error)
}

type awsClient struct {
	client   *http.Client
	endpoint string
	signer   Signer
}

// signedRequest applies the signature the the request using provided RequestSigner.
func (c *awsClient) SignedRequest(v url.Values) ([]byte, error) {
	req, err := http.NewRequest("GET", c.endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Version param is required for Amazon to understand the request.
	v.Add("Version", "2014-05-01")
	req.URL.RawQuery = v.Encode()

	c.signer.Sign(req)
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	b, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		// TODO: Prettier error reply
		return nil, errors.New(string(b))
	}

	return b, nil
}

// NewSignedRequester combines the provided http.Client with awsauth to provide a SignedRequester.
func NewSignedRequester(requester *http.Client, endpoint string, signer Signer) SignedRequester {
	c := new(awsClient)

	if endpoint == "" {
		c.endpoint = "https://ec2.amazonaws.com"
	} else {
		c.endpoint = endpoint
	}
	if requester == nil {
		c.client = http.DefaultClient
	} else {
		c.client = requester
	}
	if signer == nil {
		c.signer = DefaultSigner
	} else {
		c.signer = signer
	}

	return SignedRequester(c)
}
