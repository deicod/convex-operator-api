package httpapi

import "testing"

func TestNamespaceAllowed(t *testing.T) {
	if !namespaceAllowed(nil, "any") {
		t.Fatalf("expected open access when allowed list is empty")
	}
	if namespaceAllowed([]string{"ns1"}, "") {
		t.Fatalf("expected empty namespace to be denied when list present")
	}
	if namespaceAllowed([]string{"ns1"}, "ns2") {
		t.Fatalf("expected disallowed namespace to be denied")
	}
	if !namespaceAllowed([]string{"ns1", "ns2"}, "ns2") {
		t.Fatalf("expected allowed namespace to pass")
	}
}
