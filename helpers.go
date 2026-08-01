// LICENSE BSD-2-Clause-FreeBSD
// Copyright (c) 2018, Rohan Verma <hello@rohanverma.net>

package simples3

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// getURL constructs a URL for a given path, with multiple optional
// arguments as individual subfolders, based on the endpoint
// specified in s3 struct.
//
// When s3.UseVirtualHostedStyle is true and a custom Endpoint is configured,
// the first path segment is treated as the bucket and is prepended to the
// host as a subdomain (https://<bucket>.<endpoint>/<rest>). The remaining
// path segments become the URL path. S3-compatible services that reject
// path-style URLs (e.g. Alibaba Cloud OSS) require this form.
func (s3 *S3) getURL(path string, args ...string) (uri string) {
	if len(args) > 0 {
		path += "/" + strings.Join(args, "/")
	}
	// need to encode special characters in the path part of the URL
	encodedPath := encodePath(path)

	if len(s3.Endpoint) > 0 {
		if s3.UseVirtualHostedStyle && encodedPath != "" {
			if vhURI, ok := s3.virtualHostedURL(encodedPath); ok {
				return vhURI
			}
		}
		uri = s3.Endpoint + "/" + encodedPath
	} else {
		uri = fmt.Sprintf(s3.URIFormat, s3.Region, encodedPath)
	}

	return uri
}

// virtualHostedURL rewrites an "endpoint/bucket/key..." path into a
// virtual-hosted style URL of the form "scheme://bucket.endpoint/key...".
// Returns false when the path doesn't carry a bucket (no leading segment)
// so the caller can fall back to its default URL construction.
func (s3 *S3) virtualHostedURL(encodedPath string) (string, bool) {
	parts := strings.SplitN(encodedPath, "/", 2)
	bucket := parts[0]
	if bucket == "" {
		return "", false
	}

	scheme := "https"
	host := s3.Endpoint
	if i := strings.Index(host, "://"); i >= 0 {
		scheme = host[:i]
		host = host[i+3:]
	}
	// strip userinfo, port-less normalization isn't needed for AWS SigV4 host
	// signing — the AWS SigV4 canonical host matches the request Host header.
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}

	var b strings.Builder
	b.WriteString(scheme)
	b.WriteString("://")
	b.WriteString(bucket)
	b.WriteByte('.')
	b.WriteString(host)
	if len(parts) > 1 && parts[1] != "" {
		b.WriteByte('/')
		b.WriteString(parts[1])
	}
	return b.String(), true
}

func detectFileSize(body io.Seeker) (int64, error) {
	pos, err := body.Seek(0, 1)
	if err != nil {
		return -1, err
	}
	defer body.Seek(pos, 0)

	n, err := body.Seek(0, 2) //nolint:gomnd
	if err != nil {
		return -1, err
	}
	return n, nil
}

func getFirstString(s []string) string {
	if len(s) > 0 {
		return s[0]
	}

	return ""
}

// if object matches reserved string, no need to encode them.
var reservedObjectNames = regexp.MustCompile("^[a-zA-Z0-9-_.~/]+$")

// encodePath encode the strings from UTF-8 byte representations to HTML hex escape sequences
//
// This is necessary since regular url.Parse() and url.Encode() functions do not support UTF-8
// non english characters cannot be parsed due to the nature in which url.Encode() is written
//
// This function on the other hand is a direct replacement for url.Encode() technique to support
// pretty much every UTF-8 character.
// adapted from
// https://github.com/minio/minio-go/blob/fe1f3855b146c1b6ce4199740d317e44cf9e85c2/pkg/s3utils/utils.go#L285
func encodePath(pathName string) string {
	if reservedObjectNames.MatchString(pathName) {
		return pathName
	}
	var encodedPathname strings.Builder
	for _, s := range pathName {
		if 'A' <= s && s <= 'Z' || 'a' <= s && s <= 'z' || '0' <= s && s <= '9' { // §2.3 Unreserved characters (mark)
			encodedPathname.WriteRune(s)
			continue
		}
		switch s {
		case '-', '_', '.', '~', '/': // §2.3 Unreserved characters (mark)
			encodedPathname.WriteRune(s)
			continue
		default:
			lenR := utf8.RuneLen(s)
			if lenR < 0 {
				// if utf8 cannot convert, return the same string as is
				return pathName
			}
			u := make([]byte, lenR)
			utf8.EncodeRune(u, s)
			for _, r := range u {
				hex := hex.EncodeToString([]byte{r})
				encodedPathname.WriteString("%" + strings.ToUpper(hex))
			}
		}
	}
	return encodedPathname.String()
}

// encodeTagsHeader encodes tags as a URL-encoded key=value string for the x-amz-tagging header.
// Format: key1=value1&key2=value2
// Keys are sorted for consistency.
func encodeTagsHeader(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build encoded string
	parts := make([]string, 0, len(tags))
	for _, k := range keys {
		encodedKey := url.QueryEscape(k)
		encodedValue := url.QueryEscape(tags[k])
		parts = append(parts, encodedKey+"="+encodedValue)
	}

	return strings.Join(parts, "&")
}
