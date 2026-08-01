package simples3

import (
	"strings"
	"testing"
)

type tConfig struct {
	AccessKey string
	SecretKey string
	Endpoint  string
	Region    string
}

func TestCustomEndpoint(t *testing.T) {
	s3 := New("us-east-1", "AccessKey", "SuperSecretKey")

	// no protocol specified, should default to https
	s3.SetEndpoint("example.com")
	if s3.getURL("bucket1") != "https://example.com/bucket1" {
		t.Errorf("S3.SetEndpoint() got = %v", s3.Endpoint)
	}

	// explicit http protocol
	s3.SetEndpoint("http://localhost:9000")
	if s3.getURL("bucket2") != "http://localhost:9000/bucket2" {
		t.Errorf("S3.SetEndpoint() got = %v", s3.Endpoint)
	}

	// explicit http protocol
	s3.SetEndpoint("https://example.com")
	if s3.getURL("bucket3") != "https://example.com/bucket3" {
		t.Errorf("S3.SetEndpoint() got = %v", s3.Endpoint)
	}

	// try with trailing slash
	s3.SetEndpoint("https://example.com/foobar/")
	if s3.getURL("bucket4") != "https://example.com/foobar/bucket4" {
		t.Errorf("S3.SetEndpoint() got = %v", s3.Endpoint)
	}
}

func TestGetURL(t *testing.T) {
	s3 := New("us-east-1", "AccessKey", "SuperSecretKey")

	type args struct {
		bucket string
		params []string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "getURL: basic test",
			args: args{
				bucket: "xyz",
			},
			want: "https://s3.us-east-1.amazonaws.com/xyz",
		},
		{
			name: "getURL: multiple parameters",
			args: args{
				bucket: "xyz",
				params: []string{"hello", "world"},
			},
			want: "https://s3.us-east-1.amazonaws.com/xyz/hello/world",
		},
		{
			name: "getURL: special characters",
			args: args{
				bucket: "xyz",
				params: []string{"hello, world!", "#!@$%^&*(1).txt"},
			},
			want: "https://s3.us-east-1.amazonaws.com/xyz/hello%2C%20world%21/%23%21%40%24%25%5E%26%2A%281%29.txt",
		},
	}

	for _, testcase := range tests {
		tt := testcase
		t.Run(tt.name, func(t *testing.T) {
			url := s3.getURL(tt.args.bucket, tt.args.params...)
			if url != tt.want {
				t.Errorf("S3.getURL() got = %v, want %v", url, tt.want)
			}
		})
	}
}

func TestVirtualHostedStyle(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		path     string
		args     []string
		want     string
	}{
		{
			name:     "OSS-style: bucket + key with custom endpoint, no scheme",
			endpoint: "oss-cn-hangzhou.aliyuncs.com",
			path:     "mybucket",
			args:     []string{"path/to/key.txt"},
			want:     "https://mybucket.oss-cn-hangzhou.aliyuncs.com/path/to/key.txt",
		},
		{
			name:     "OSS-style: bucket only (e.g. ListObjects)",
			endpoint: "oss-cn-hangzhou.aliyuncs.com",
			path:     "mybucket",
			want:     "https://mybucket.oss-cn-hangzhou.aliyuncs.com",
		},
		{
			name:     "explicit https scheme preserved",
			endpoint: "https://oss-cn-hangzhou.aliyuncs.com",
			path:     "mybucket",
			want:     "https://mybucket.oss-cn-hangzhou.aliyuncs.com",
		},
		{
			name:     "explicit http scheme preserved (for local MinIO)",
			endpoint: "http://localhost:9000",
			path:     "mybucket",
			want:     "http://mybucket.localhost:9000",
		},
		{
			name:     "trailing path on endpoint stripped before subdomain",
			endpoint: "https://oss-cn-hangzhou.aliyuncs.com/extra/",
			path:     "mybucket",
			want:     "https://mybucket.oss-cn-hangzhou.aliyuncs.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3 := New("cn-hangzhou", "AccessKey", "SecretKey").
				SetEndpoint(tt.endpoint).
				SetVirtualHostedStyle(true)
			got := s3.getURL(tt.path, tt.args...)
			if got != tt.want {
				t.Errorf("virtual-hosted getURL() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVirtualHostedStyleDefaultOff(t *testing.T) {
	// Without SetVirtualHostedStyle(true) we must keep the original
	// path-style behaviour, so the S3-SDK upgrade stays non-breaking.
	s3 := New("us-east-1", "AccessKey", "SecretKey").
		SetEndpoint("https://oss-cn-hangzhou.aliyuncs.com")
	if got := s3.getURL("mybucket", "k.txt"); got != "https://oss-cn-hangzhou.aliyuncs.com/mybucket/k.txt" {
		t.Errorf("default path-style broken: got %q", got)
	}
}

func TestVirtualHostedStyleEmptyPath(t *testing.T) {
	// When path is empty, virtual-hosted style can't be applied
	// (no bucket to use as subdomain). Caller still gets the original
	// path-style URL so ListBuckets-style operations still work.
	s3 := New("us-east-1", "AccessKey", "SecretKey").
		SetEndpoint("https://oss-cn-hangzhou.aliyuncs.com").
		SetVirtualHostedStyle(true)
	if got := s3.getURL(""); got != "https://oss-cn-hangzhou.aliyuncs.com/" {
		t.Errorf("empty path should fall back to path-style: got %q", got)
	}
}

// Helper functions
func uploadTestFiles(t *testing.T, s3 *S3, bucket string, filenames []string) {
	for _, filename := range filenames {
		content := strings.NewReader("test content for " + filename)
		_, err := s3.FilePut(UploadInput{
			Bucket:      bucket,
			ObjectKey:   filename,
			ContentType: "text/plain",
			Body:        content,
		})
		if err != nil {
			t.Fatalf("Failed to upload test file %s: %v", filename, err)
		}
	}
}

func cleanupTestFiles(t *testing.T, s3 *S3, bucket string, filenames []string) {
	for _, filename := range filenames {
		err := s3.FileDelete(DeleteInput{
			Bucket:    bucket,
			ObjectKey: filename,
		})
		if err != nil {
			t.Logf("Failed to cleanup test file %s: %v", filename, err)
		}
	}
}
