package acl

import (
	"testing"
)

func TestParseACE(t *testing.T) {
	aceStr := "A:fd:kelvdun@dccn.nl:rwaDdxtTnNcy"
	ace, _ := parseAce(aceStr)
	if ace.Principle != "kelvdun@dccn.nl" {
		t.Errorf("Expected principle %s but got %s", "kelvdun@dccn.nl", ace.Principle)
	}
}

func TestGetPrincipleName(t *testing.T) {
	aceStr := "A:fd:kelvdun@dccn.nl:rwaDdxtTnNcy"
	ace, _ := parseAce(aceStr)
	pn := getPrincipleName(*ace)
	if pn != "kelvdun" {
		t.Errorf("Expected principle name %s but got %s", "kelvdun", pn)
	}
}

func TestIsValidPrinciple(t *testing.T) {
	aceStr := "A:fd:rendbru@dccn.nl:rwaDdxtTnNcy"
	ace, _ := parseAce(aceStr)
	if !ace.IsValidPrinciple() {
		t.Errorf("principle not valid: %s", ace.Principle)
	}
}

func TestAceToContributor(t *testing.T) {
	aceStr := "A:fd:kelvdun@dccn.nl:rwadxtTnNcy"
	ace, _ := parseAce(aceStr)

	// do it multiple times to make sure there is no ambiguity
	for i := 1; i <= 10; i++ {
		r := ace.ToRole()
		if r != Contributor {
			t.Errorf("iteration %d: Expected role %s but got %s", i, Contributor, r)
		}
	}
}

func TestAceToWriter(t *testing.T) {
	aceStr := "A:fd:kelvdun@dccn.nl:rwaxnNtTcy"
	ace, _ := parseAce(aceStr)

	// do it multiple times to make sure there is no ambiguity
	for i := 1; i <= 10; i++ {
		r := ace.ToRole()
		if r != Writer {
			t.Errorf("iteration %d: Expected role %s but got %s", i, Writer, r)
		}
	}
}
