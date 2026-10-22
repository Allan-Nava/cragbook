package memory

import (
	"testing"

	"github.com/Allan-Nava/cragbook/internal/infrastructure/contract"
)

func TestTheContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Store { return New() })
}
