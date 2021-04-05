package cloudinit

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/textproto"
)

type PartReader interface {
	Filename() string
	Content() string
	ContentType() string
	MergeType() string
}

// CloudConfiger
type CloudConfiger interface {
	UseGzipCompression() bool
	UseBase64Encoding() bool
	GetParts() []PartReader
	Base64Boundary() string
}

// RenderCloudinitConfig renders a CloudConfiger to string, with the base64, gzip encoding, and parts settings defined in the CloudConfiger object
func RenderCloudinitConfig(d CloudConfiger) (string, error) {
	gzipOutput := d.UseGzipCompression()
	base64Output := d.UseBase64Encoding()
	mimeBoundary := d.Base64Boundary()

	if gzipOutput && !base64Output {
		return "", fmt.Errorf("base64_encode is mandatory when gzip is enabled")
	}

	partsValue := d.GetParts()
	hasParts := len(partsValue) > 0
	if !hasParts {
		return "", fmt.Errorf("No parts found in the cloudinit resource declaration")
	}

	var buffer bytes.Buffer

	var err error
	if gzipOutput {
		gzipWriter := gzip.NewWriter(&buffer)
		err = renderPartsToWriter(mimeBoundary, partsValue, gzipWriter)
		err = gzipWriter.Close()
		if err != nil {
			return "", err
		}
	} else {
		err = renderPartsToWriter(mimeBoundary, partsValue, &buffer)
	}
	if err != nil {
		return "", err
	}

	output := ""
	if base64Output {
		output = base64.StdEncoding.EncodeToString(buffer.Bytes())
	} else {
		output = buffer.String()
	}

	return output, nil
}

func renderPartsToWriter(mimeBoundary string, parts []PartReader, writer io.Writer) error {
	mimeWriter := multipart.NewWriter(writer)
	defer func() {
		err := mimeWriter.Close()
		if err != nil {
			log.Printf("[WARN] Error closing mimewriter: %s", err)
		}
	}()

	// we need to set the boundary explictly, otherwise the boundary is random
	// and this causes terraform to complain about the resource being different
	if err := mimeWriter.SetBoundary(mimeBoundary); err != nil {
		return err
	}

	writer.Write([]byte(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\n", mimeWriter.Boundary())))
	writer.Write([]byte("MIME-Version: 1.0\r\n\r\n"))

	for _, part := range parts {
		header := textproto.MIMEHeader{}
		if part.ContentType() == "" {
			header.Set("Content-Type", "text/plain")
		} else {
			header.Set("Content-Type", part.ContentType())
		}

		header.Set("MIME-Version", "1.0")
		header.Set("Content-Transfer-Encoding", "7bit")

		if part.Filename() != "" {
			header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, part.Filename()))
		}

		if part.MergeType() != "" {
			header.Set("X-Merge-Type", part.MergeType())
		}

		partWriter, err := mimeWriter.CreatePart(header)
		if err != nil {
			return err
		}

		_, err = partWriter.Write([]byte(part.Content()))
		if err != nil {
			return err
		}
	}

	return nil
}