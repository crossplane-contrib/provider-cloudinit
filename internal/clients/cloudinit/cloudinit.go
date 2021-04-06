package cloudinit

import (
	model "github.com/crossplane-contrib/provider-cloudinit/internal/cloudinit"
)

type part struct {
	filename    string
	content     string
	contentType string
	mergeType   string
}

type ClientConfig struct {
	Base64Boundary     string
	UseGzipCompression bool
	UseBase64Encoding  bool
	Parts              []model.PartReader
}

type Client struct {
	*ClientConfig
}

var (
	_ model.CloudConfiger = (*Client)(nil)
	_ model.PartReader    = (*part)(nil)
)

func NewClient(useGzipCompression bool, useBase64Encoding bool, base64Boundary string) *Client {
	return &Client{
		&ClientConfig{
			Base64Boundary:     base64Boundary,
			UseGzipCompression: useGzipCompression,
			UseBase64Encoding:  useBase64Encoding,
		},
	}
}

func (c *ClientConfig) NewPart(content, filename, contentType, mergeType string) *part {
	return &part{
		filename:    filename,
		content:     content,
		contentType: contentType,
		mergeType:   mergeType,
	}
}

func (c *ClientConfig) AppendPart(content, filename, contentType, mergeType string) {
	c.Parts = append(c.Parts, c.NewPart(content, filename, contentType, mergeType))
}

func (c *ClientConfig) GetParts() []model.PartReader {
	return c.Parts
}

func (p *part) Filename() string    { return p.filename }
func (p *part) Content() string     { return p.content }
func (p *part) ContentType() string { return p.contentType }
func (p *part) MergeType() string   { return p.mergeType }

func (c *Client) Base64Boundary() string {
	return c.ClientConfig.Base64Boundary
}

func (c *Client) UseGzipCompression() bool {
	return c.ClientConfig.UseGzipCompression
}
func (c *Client) UseBase64Encoding() bool {
	return c.ClientConfig.UseBase64Encoding
}
