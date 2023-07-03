// Package rgwadmin is a small, incomplete and self-contained client for
// Ceph's radosgw admin API.
//
// It is written to serve terraform-provider-radosgw and only implements
// the operations necessary for that use-case for now.
//
// See https://docs.ceph.com/en/latest/radosgw/adminops/ for details on
// the API.
package rgwadmin

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	// "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

type Client interface {
	Info(ctx context.Context) error
}

type client struct {
	endpoint        string
	accessKeyID     string
	secretAccessKey string

	signer v4.Signer
}

func New(endpoint, accessKeyID, secretAccessKey string) Client {
	return &client{
		endpoint:        endpoint,
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,

		signer: *v4.NewSigner(func(opts *v4.SignerOptions) {
			opts.DisableURIPathEscaping = true
		}),
	}
}

func (c *client) Info(ctx context.Context) error {
	query := make(url.Values)
	query.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/admin/usage?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	req = Sign(req, c.accessKeyID, c.secretAccessKey)
	// err = c.signer.SignHTTP(ctx, aws.Credentials{AccessKeyID: c.accessKeyID, SecretAccessKey: c.secretAccessKey}, req, emptyPayloadHash, "", "", time.Now().UTC())
	// if err != nil {
	// 	return fmt.Errorf("could not sign request: %w", err)
	// }

	// return fmt.Errorf("%s %s %s %s %s", req.Method, req.URL, req.Header.Get("Authorization"), req.Header.Get("X-Amz-Date"), req.Header.Get("Host"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response code %d: %s", resp.StatusCode, data)
	}

	var info struct {
		Info struct {
			ClusterID string `json:"cluster_id"`
		} `json:"info"`
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&info)
	if err != nil {
		return fmt.Errorf("could not parse json: %w", err)
	}

	if info.Info.ClusterID == "" {
		return fmt.Errorf("invalid info, info.cluster_id is empty")
	}

	return nil
}

// Sign signs the request using the S3 v4 signature algorithm.
func Sign(req *http.Request, accessKeyID, secretAccessKey string) *http.Request {
	now := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05") + " GMT"
	mac := hmac.New(sha1.New, []byte(secretAccessKey))
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", req.Method, "", "", now, req.URL.Path)
	mac.Write([]byte(stringToSign))
	hmac := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("Date", now)
	req.Header.Set("Authorization", "AWS "+accessKeyID+":"+hmac)

	return req
}
