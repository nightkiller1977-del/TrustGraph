package verification

import "encoding/base64"

// encodeBase64 is the shared image-encoding helper for vendor payloads.
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
