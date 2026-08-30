package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSettingsValidateDefaults(t *testing.T) {
	if err := newSettings(nil).Validate(); err != nil {
		t.Fatalf("defaults should validate: %v", err)
	}
}

func TestSettingsFieldsAcceptJK(t *testing.T) {
	f := newSettings(nil)
	f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jk")})
	if got := f.inputs[0].Value(); got != "jk" {
		t.Fatalf("typed value = %q, want jk", got)
	}
	if f.focus != 0 {
		t.Fatalf("typing j/k moved focus to %d", f.focus)
	}
}

func TestSettingsValidateRejectsInvalidPort(t *testing.T) {
	f := newSettings(nil)
	f.inputs[2].SetValue("70000")
	if err := f.Validate(); err == nil {
		t.Fatal("expected invalid auth port to be rejected")
	}
}

func TestSettingsValidateRejectsDuplicatePorts(t *testing.T) {
	f := newSettings(nil)
	f.inputs[2].SetValue("8090")
	f.inputs[3].SetValue("8090")
	if err := f.Validate(); err == nil {
		t.Fatal("expected duplicate daemon ports to be rejected")
	}
}

func TestSettingsValidateRejectsBotRange(t *testing.T) {
	f := newSettings(nil)
	f.inputs[4].SetValue("30")
	f.inputs[5].SetValue("10")
	if err := f.Validate(); err == nil {
		t.Fatal("expected inverted bot range to be rejected")
	}
}

func TestSettingsValidateMapsURL(t *testing.T) {
	f := newSettings(nil)
	f.inputs[12].SetValue("not a URL")
	if err := f.Validate(); err == nil {
		t.Fatal("expected invalid maps URL to be rejected")
	}
	f.inputs[12].SetValue("https://example.test/maps.zip")
	if err := f.Validate(); err != nil {
		t.Fatalf("expected valid maps URL: %v", err)
	}
}

func TestSettingsValidateRejectsConfDelimitersInPassword(t *testing.T) {
	f := newSettings(nil)
	f.inputs[8].SetValue(`bad;password`)
	if err := f.Validate(); err == nil {
		t.Fatal("expected semicolon-delimited database password to be rejected")
	}
	f.inputs[8].SetValue(`bad"password`)
	if err := f.Validate(); err == nil {
		t.Fatal("expected quoted database password to be rejected")
	}
}
