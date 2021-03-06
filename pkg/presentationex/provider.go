/*
Copyright SecureKey Technologies Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package presentationex

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/hyperledger/aries-framework-go/pkg/doc/presexch"
)

// Provider provide presentation exchange ops.
type Provider struct {
	inputDescriptors map[string]*presexch.InputDescriptor
}

// New return new provider for presentation exchange.
func New(inputDescriptorsFile io.Reader) (*Provider, error) {
	p := &Provider{
		inputDescriptors: make(map[string]*presexch.InputDescriptor),
	}

	data, err := ioutil.ReadAll(inputDescriptorsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read input descriptors file: %w", err)
	}

	if err := json.Unmarshal(data, &p.inputDescriptors); err != nil {
		return nil, fmt.Errorf("failed unmarshal to input descriptors %w", err)
	}

	return p, nil
}

// Create presentation exchange request.
func (p *Provider) Create(scopes []string) (*presexch.PresentationDefinition, error) {
	defs := &presexch.PresentationDefinition{
		ID:               uuid.New().String(),
		InputDescriptors: make([]*presexch.InputDescriptor, 0),
	}

	for _, scope := range scopes {
		def, found := p.inputDescriptors[scope]
		if !found {
			return nil, fmt.Errorf("scope [%s] not supported", scope)
		}

		def.ID = scope
		defs.InputDescriptors = append(defs.InputDescriptors, def)
	}

	err := defs.ValidateSchema()
	if err != nil {
		return nil, fmt.Errorf("presentation_definition jsonschema validation failure: %w", err)
	}

	return defs, nil
}
