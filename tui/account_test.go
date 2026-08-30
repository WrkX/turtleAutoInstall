package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWowPassHashKnownVector(t *testing.T) {
	got := wowPassHash("admin", "admin")
	want := "8301316d0d8448a34fa6d0c6bf1cbfa2b4a1a93a"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if wowPassHash("ADMIN", "ADMIN") != want {
		t.Fatal("hash must be case-insensitive")
	}
}

func TestValidateAccountName(t *testing.T) {
	if err := validateAccountName("ab"); err != nil {
		t.Fatal(err)
	}
	if err := validateAccountName("Player_1"); err == nil {
		t.Fatal("underscore should be rejected")
	}
	if err := validateAccountName("x"); err == nil {
		t.Fatal("too short")
	}
	if err := validateAccountName("abcdefghijklmnopq"); err == nil {
		t.Fatal("too long")
	}
}

func TestAccountFormValidation(t *testing.T) {
	f := newAccountForm()
	f.inputs[0].SetValue("Player1")
	f.inputs[1].SetValue("secret")
	if user, hash, gm, err := f.validate(); err != nil || user != "Player1" || hash == "" || gm != "0" {
		t.Fatalf("valid form = %q, %q, %q, %v", user, hash, gm, err)
	}

	f.inputs[2].SetValue("4")
	if err := func() error { _, _, _, err := f.validate(); return err }(); err == nil {
		t.Fatal("GM level 4 should be rejected")
	}
	f.inputs[2].SetValue("3")
	f.inputs[1].SetValue("")
	if err := func() error { _, _, _, err := f.validate(); return err }(); err == nil {
		t.Fatal("empty password should be rejected")
	}
}

func TestAccountFieldsAcceptJK(t *testing.T) {
	f := newAccountForm()
	f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jk")})
	if got := f.inputs[0].Value(); got != "jk" {
		t.Fatalf("typed value = %q, want jk", got)
	}
	if f.focus != 0 {
		t.Fatalf("typing j/k moved focus to %d", f.focus)
	}
}
