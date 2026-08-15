package processor

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/afbackend/pretrade-risk-engine/internal/account"
)

func TestDefault(t *testing.T) {
	tests := []struct {
		name   string
		ID     int64
		Status account.RiskStatus
		Reason account.RiskReason
	}{
		{"Simple Accepted", 1, account.Accepted, account.None},
		{"Simple Rejected", 2, account.Rejected, account.InsufficientBuyingPower},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			rResult := &account.RiskResult{OrderID: tt.ID, Status: tt.Status, Reason: tt.Reason}
			want, err := json.Marshal(rResult)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want = append(want, '\n')

			buf := bytes.Buffer{}
			p := &DefaultProcessor{buffer: &buf}

			p.RiskResult(rResult)

			got := buf.Bytes()

			if !bytes.Equal(got, want) {
				t.Fatalf("expected %s, got %s", string(got), string(want))
			}

		})
	}
}

func TestMultipleResults(t *testing.T) {
	results := []*account.RiskResult{
		{OrderID: 1, Status: account.Accepted, Reason: account.None},
		{OrderID: 2, Status: account.Rejected, Reason: account.InsufficientBuyingPower},
		{OrderID: 3, Status: account.Accepted, Reason: account.None},
	}

	buf := bytes.Buffer{}
	p := &DefaultProcessor{buffer: &buf}

	var want []byte
	for _, r := range results {
		p.RiskResult(r)

		line, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want = append(want, line...)
		want = append(want, '\n')
	}

	got := buf.Bytes()

	if !bytes.Equal(got, want) {
		t.Fatalf("expected %s, got %s", string(want), string(got))
	}
}
