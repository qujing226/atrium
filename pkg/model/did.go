package model

import (
	"encoding/json"
	"fmt"

	"github.com/multiformats/go-multibase"
	ssi "github.com/nuts-foundation/go-did"
	"github.com/nuts-foundation/go-did/did"
)

const (
	Dilithium3KeyType  ssi.KeyType = "Dilithium3PublicKey"
	Kyber768KeyType ssi.KeyType = "Kyber768PublicKey"
)

// DIDDocumentWrapper provides utility functions around the standard W3C DID document.
type DIDDocumentWrapper struct {
	*did.Document
}

// BuildDIDDocument creates a standard W3C DID Document from raw public keys.
func BuildDIDDocument(didID string, signPk []byte, kyberPk []byte) ([]byte, error) {
	parsedDID, err := did.ParseDID(didID)
	if err != nil {
		return nil, fmt.Errorf("invalid DID ID: %w", err)
	}

	doc := &did.Document{
		Context: []interface{}{"https://www.w3.org/ns/did/v1"},
		ID:      *parsedDID,
	}

	// Add Dilithium3 Verification Method
	signMb, _ := multibase.Encode(multibase.Base58BTC, signPk)
	signURL := did.MustParseDIDURL(didID + "#sign-1")
	signVM := &did.VerificationMethod{
		ID:                 signURL,
		Type:               Dilithium3KeyType,
		Controller:         *parsedDID,
		PublicKeyMultibase: signMb,
	}
	doc.VerificationMethod = append(doc.VerificationMethod, signVM)
	doc.Authentication.Add(signVM)

	// Add Kyber768 Verification Method
	kyberMb, _ := multibase.Encode(multibase.Base58BTC, kyberPk)
	kyberURL := did.MustParseDIDURL(didID + "#kyber-1")
	kyberVM := &did.VerificationMethod{
		ID:                 kyberURL,
		Type:               Kyber768KeyType,
		Controller:         *parsedDID,
		PublicKeyMultibase: kyberMb,
	}
	doc.VerificationMethod = append(doc.VerificationMethod, kyberVM)
	doc.KeyAgreement.Add(kyberVM)

	return json.MarshalIndent(doc, "", "  ")
}

// ParseDIDDocument parses a raw JSON byte slice into a DIDDocumentWrapper.
func ParseDIDDocument(raw []byte) (*DIDDocumentWrapper, error) {
	doc, err := did.ParseDocument(string(raw))
	if err != nil {
		return nil, err
	}
	return &DIDDocumentWrapper{Document: doc}, nil
}

// GetKyberPubKey extracts the Kyber768 public key bytes from the DID Document.
func (w *DIDDocumentWrapper) GetKyberPubKey() ([]byte, error) {
	for _, vm := range w.VerificationMethod {
		if vm.Type == Kyber768KeyType && vm.PublicKeyMultibase != "" {
			_, decoded, err := multibase.Decode(vm.PublicKeyMultibase)
			return decoded, err
		}
	}
	return nil, fmt.Errorf("kyber key not found in DID document")
}

// GetSigningPubKey extracts the Ed25519 public key bytes from the DID Document.
func (w *DIDDocumentWrapper) GetSigningPubKey() ([]byte, error) {
	for _, vm := range w.VerificationMethod {
		if vm.Type == Dilithium3KeyType && vm.PublicKeyMultibase != "" {
			_, decoded, err := multibase.Decode(vm.PublicKeyMultibase)
			return decoded, err
		}
	}
	return nil, fmt.Errorf("signing key not found in DID document")
}
