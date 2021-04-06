package cloudinit

import (
	model "github.com/crossplane-contrib/provider-cloudinit/internal/cloudinit"
)

// part defines part of a multi-part mime document
type part struct {
	filename    string
	content     string
	contentType string
	mergeType   string
}

// ClientConfig defines the properties needed to encode parts as multi-part mime
type ClientConfig struct {
	Base64Boundary     string
	UseGzipCompression bool
	UseBase64Encoding  bool
	Parts              []model.PartReader
}

// Client stores the client config and implements CloudConfiger
type Client struct {
	*ClientConfig
}

var (
	_ model.CloudConfiger = (*Client)(nil)
	_ model.PartReader    = (*part)(nil)
)

// NewClient creates a new client
func NewClient(useGzipCompression bool, useBase64Encoding bool, base64Boundary string) *Client {
	return &Client{
		&ClientConfig{
			Base64Boundary:     base64Boundary,
			UseGzipCompression: useGzipCompression,
			UseBase64Encoding:  useBase64Encoding,
		},
	}
}

// newPart constructs and returns a new part
func (c *ClientConfig) newPart(content, filename, contentType, mergeType string) *part {
	return &part{
		filename:    filename,
		content:     content,
		contentType: contentType,
		mergeType:   mergeType,
	}
}

// AppendPart appends a new part to the client config
func (c *ClientConfig) AppendPart(content, filename, contentType, mergeType string) {
	c.Parts = append(c.Parts, c.newPart(content, filename, contentType, mergeType))
}

// GetParts returns the parts of the client config
func (c *ClientConfig) GetParts() []model.PartReader {
	return c.Parts
}

// Filename is the filename of the part
func (p *part) Filename() string { return p.filename }

// Content is the content of the part
func (p *part) Content() string { return p.content }

// ContentType is the content-type of the part
func (p *part) ContentType() string { return p.contentType }

// MergeType is the merge-type of the part
func (p *part) MergeType() string { return p.mergeType }

// Base64Boundary is the base64 boundary that will be used
func (c *Client) Base64Boundary() string {
	return c.ClientConfig.Base64Boundary
}

// UseGzipCompression indicates if gzip compression will be used
func (c *Client) UseGzipCompression() bool {
	return c.ClientConfig.UseGzipCompression
}

// UseBase64Encoding indicates if base64 encoding will be used
func (c *Client) UseBase64Encoding() bool {
	return c.ClientConfig.UseBase64Encoding
}
