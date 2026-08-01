// LICENSE BSD-2-Clause-FreeBSD
// Copyright (c) 2018, Rohan Verma <hello@rohanverma.net>

package simples3

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// AMZMetaPrefix to prefix metadata key.
	AMZMetaPrefix = "x-amz-meta-"
)

// S3 provides a wrapper around your S3 credentials.
type S3 struct {
	AccessKey string
	SecretKey string
	Region    string
	Client    *http.Client

	Token     string
	Endpoint  string
	URIFormat string
	// UseVirtualHostedStyle enables virtual-hosted style addressing when a
	// custom Endpoint is set. In virtual-hosted style the bucket name is
	// embedded as a subdomain of the endpoint
	// (e.g. https://mybucket.oss-cn-hangzhou.aliyuncs.com/key) instead of
	// appearing in the URL path (https://oss-cn-hangzhou.aliyuncs.com/mybucket/key).
	// S3-compatible services such as Alibaba Cloud OSS, Tencent COS and
	// some private clouds reject path-style requests with
	// "SecondLevelDomainForbidden", so this flag is required for them.
	UseVirtualHostedStyle bool
	initMode              string
	expiry                time.Time

	mu sync.Mutex
}

// New returns an instance of S3.
func New(region, accessKey, secretKey string) *S3 {
	return &S3{
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,

		URIFormat: "https://s3.%s.amazonaws.com/%s",
	}
}

func (s3 *S3) getClient() *http.Client {
	if s3.Client == nil {
		return http.DefaultClient
	}
	return s3.Client
}

// SetEndpoint can be used to the set a custom endpoint for
// using an alternate instance compatible with the s3 API.
// If no protocol is included in the URI, defaults to HTTPS.
func (s3 *S3) SetEndpoint(uri string) *S3 {
	if len(uri) > 0 {
		if !strings.HasPrefix(uri, "http") {
			uri = "https://" + uri
		}

		// make sure there is no trailing slash
		if uri[len(uri)-1] == '/' {
			uri = uri[:len(uri)-1]
		}
		s3.Endpoint = uri
	}
	return s3
}

// SetToken can be used to set a Temporary Security Credential token obtained from
// using an IAM role or AWS STS.
func (s3 *S3) SetToken(token string) *S3 {
	if token != "" {
		s3.Token = token
	}
	return s3
}

// SetClient can be used to set the http client to be
// used by the package. If client passed is nil,
// http.DefaultClient is used.
func (s3 *S3) SetClient(client *http.Client) *S3 {
	if client != nil {
		s3.Client = client
	} else {
		s3.Client = http.DefaultClient
	}
	return s3
}

// SetVirtualHostedStyle toggles virtual-hosted style URL addressing for S3
// operations against a custom Endpoint. When enabled, the bucket name is
// embedded as a subdomain of the endpoint
// (https://mybucket.<endpoint>/<key>). S3-compatible services that reject
// path-style URLs (Alibaba Cloud OSS, Tencent COS, Huawei OBS, etc.) need
// this enabled. AWS S3 supports both styles; this is a no-op when no custom
// Endpoint is set.
func (s3 *S3) SetVirtualHostedStyle(enabled bool) *S3 {
	s3.UseVirtualHostedStyle = enabled
	return s3
}

func (s3 *S3) signRequest(req *http.Request) error {
	var (
		err error

		date = req.Header.Get("Date")
		t    = time.Now().UTC()
	)

	if date != "" {
		t, err = time.Parse(http.TimeFormat, date)
		if err != nil {
			return err
		}
	}
	req.Header.Set("Date", t.Format(amzDateISO8601TimeFormat))
	req.Header.Set("X-Amz-Date", t.Format(amzDateISO8601TimeFormat))

	if s3.Token != "" {
		req.Header.Set("X-Amz-Security-Token", s3.Token)
	}

	// The x-amz-content-sha256 header is required for all AWS
	// Signature Version 4 requests. It provides a hash of the
	// request payload. If there is no payload, you must provide
	// the hash of an empty string.

	if req.Header.Get("x-amz-content-sha256") == "" {
		emptyhash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		req.Header.Set("x-amz-content-sha256", emptyhash)
	}

	k := s3.signKeys(t)
	h := hmac.New(sha256.New, k)

	s3.writeStringToSign(h, t, req)

	auth := bytes.NewBufferString(algorithm)
	auth.Write([]byte(" Credential=" + s3.AccessKey + "/" + s3.creds(t)))
	auth.Write([]byte{',', ' '})
	auth.Write([]byte("SignedHeaders="))
	writeHeaderList(auth, req)
	auth.Write([]byte{',', ' '})
	auth.Write([]byte("Signature=" + fmt.Sprintf("%x", h.Sum(nil))))

	req.Header.Set("Authorization", auth.String())
	return nil
}
