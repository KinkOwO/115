package main

import "testing"

func TestValidateArgumentsRequiresOutputAndRejectsBase(t *testing.T) {
	if err := validateArguments("", ""); err == nil {
		t.Fatal("missing output accepted")
	}
	if err := validateArguments("quest-equipment.next29.json", "out.json"); err == nil {
		t.Fatal("legacy JSON seed accepted")
	}
	if err := validateArguments("", "out.json"); err != nil {
		t.Fatal(err)
	}
}
