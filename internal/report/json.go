package report

import (
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// JSON is the exact compact UTF-8 export representation, without a trailing newline.
// The PDF digest refers to these bytes; changes in whitespace change the digest.
func JSON(r inspection.Report) ([]byte, error) { return json.Marshal(r) }
