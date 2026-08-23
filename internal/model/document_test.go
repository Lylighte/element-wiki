package model

import "testing"

func TestVisibilityValid(t *testing.T) {
	if !VisibilityStandard.Valid() || !VisibilityRestricted.Valid() {
		t.Error("两档合法值必须通过")
	}
	for _, bad := range []Visibility{"", "public", "Standard"} {
		if bad.Valid() {
			t.Errorf("%q 不应合法", bad)
		}
	}
}

func TestDocumentAlive(t *testing.T) {
	d := Document{}
	if !d.Alive() {
		t.Error("无 DeletedAt 应存活")
	}
	ts := int64(123)
	d.DeletedAt = &ts
	if d.Alive() {
		t.Error("有 DeletedAt 不应存活")
	}
}
