package devicelog

import (
	"encoding/base64"
	"io"
	"path/filepath"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

// ResultSchema identifies a structured verbatim device log retrieval.
const ResultSchema = "gopdsdk-device-log/v1"

type StructuredResult struct {
	Schema   string      `json:"schema"`
	Metadata LogMetadata `json:"metadata"`
	Content  LogContent  `json:"content"`
}

type LogMetadata struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	ByteCount int    `json:"byteCount"`
}

type LogContent struct {
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

func writeStructuredResult(out io.Writer, command string, kind Kind, path string, contents []byte) error {
	path = filepath.ToSlash(strings.ReplaceAll(path, `\`, "/"))
	result := StructuredResult{Schema: ResultSchema,
		Metadata: LogMetadata{Kind: string(kind), Path: path, ByteCount: len(contents)},
		Content:  LogContent{Encoding: "base64", Data: base64.StdEncoding.EncodeToString(contents)},
	}
	return toolingprotocol.WriteResult(out, command, result)
}
