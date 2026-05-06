package tests

import (
	"fmt"
	"testing"

	"github.com/nuts-foundation/go-did/did"
)

func TestParseDidDocument(t *testing.T) {
	rawJSON := `{
		"@context": ["https://www.w3.org/ns/did/v1"],
		"id": "did:example:123",
		"verificationMethod": [
			{
				"id": "did:example:123#key-1",
				"type": "Ed25519VerificationKey2020",
				"controller": "did:example:123",
				"publicKeyBase58": "H3C2AVvLMv6gmMNam3uVAjZpfkcJCwDwnZn6z3wXmqPV"
			},
			{
				"id": "did:example:123#key-2",
				"type": "Kyber768PublicKey",
				"controller": "did:example:123",
				"publicKeyBase58": "zxyz"
			}
		]
	}`

	doc, err := did.ParseDocument(rawJSON)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	t.Log("Parsed:", len(doc.VerificationMethod), "verification methods")
	for _, vm := range doc.VerificationMethod {
		t.Logf("Type: %s\n", vm.Type)
	}
}
