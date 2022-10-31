package strings

import (
	"testing"
)

func TestStringXOR(t *testing.T) {
	ace := "rwadxtTnNcy"
	aceWriter := "rwaxnNtTcy"
	aceContributor := "rwaDdxnNtTcy"

	lWriter := len(StringXOR(ace, aceWriter))
	lContributor := len(StringXOR(ace, aceContributor))

	t.Logf("XOR for contributor: %d", lContributor)
	t.Logf("XOR for writer: %d", lWriter)
}
