package acl

import (
    "testing"
)

func TestParseACE(t *testing.T) {
    ace_str := "A:fd:kelvdun@dccn.nl:rwaDdxtTnNcy"
    ace, _ := parseAce(ace_str)
    if ace.Principle != "kelvdun@dccn.nl" {
        t.Errorf("Expected principle %s but got %s", "kelvdun@dccn.nl", ace.Principle)
    }
}

func TestGetPrincipleName(t *testing.T) {
    ace_str := "A:fd:kelvdun@dccn.nl:rwaDdxtTnNcy"
    ace, _ := parseAce(ace_str)
    pn := getPrincipleName(*ace)
    if pn != "kelvdun" {
        t.Errorf("Expected principle name %s but got %s", "kelvdun", pn)
    }
}

func TestIsValidPrinciple(t *testing.T) {
    ace_str := "A:fd:rendbru@dccn.nl:rwaDdxtTnNcy"
    ace, _ := parseAce(ace_str)
    if ! ace.IsValidPrinciple() {
        t.Errorf("principle not valid: %s", ace.Principle)
    }
}
