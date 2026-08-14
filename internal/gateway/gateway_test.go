package gateway

import (
	"reflect"
	"testing"

	"github.com/afbackend/order-execution-engine/internal/account"
)

type SpyProcessor struct {
	results []account.RiskResult
}

func (s *SpyProcessor) RiskResult(result *account.RiskResult) error {
	s.results = append(s.results, *result)
	return nil
}

func TestRisk(t *testing.T) {
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
			want := []account.RiskResult{*rResult}

			spy := SpyProcessor{}
			g := NewGateway(nil, &spy)
			g.process(rResult)

			got := spy.results

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Expected %+v, got %+v", want, got)
			}
		})
	}

	t.Run("Mult results", func(t *testing.T) {
		ResultRejected := &account.RiskResult{OrderID: 1, Status: account.Rejected, Reason: account.InsufficientBuyingPower}
		ResultAccepted := &account.RiskResult{OrderID: 1, Status: account.Accepted, Reason: account.None}
		want := []account.RiskResult{*ResultRejected, *ResultAccepted}

		spy := SpyProcessor{}
		g := NewGateway(nil, &spy)
		g.process(ResultRejected)
		g.process(ResultAccepted)

		got := spy.results

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Expected %+v, got %+v", want, got)
		}
	})
}
