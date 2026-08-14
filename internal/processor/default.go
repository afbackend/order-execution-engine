package processor

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/afbackend/order-execution-engine/internal/account"
)

type RiskProcessor interface {
	RiskResult(result *account.RiskResult) error
}

type DefaultProcessor struct {
	buffer io.Writer
}

func (d *DefaultProcessor) RiskResult(result *account.RiskResult) error {
	buf, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal risk result: %w", err)
	}

	buf = append(buf, '\n') // delimitador → JSON Lines
	if _, err := d.buffer.Write(buf); err != nil {
		return fmt.Errorf("write risk result: %w", err)
	}

	return nil
}
