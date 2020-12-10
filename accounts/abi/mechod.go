// Copyright 2015 The go-etvchaineum Authors
// This file is part of the go-etvchaineum library.
//
// The go-etvchaineum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-etvchaineum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-etvchaineum library. If not, see <http://www.gnu.org/licenses/>.

package abi

import (
	"fmt"
	"strings"

	"github.com/etvchaineum/go-etvchaineum/crypto"
)

// Mechod represents a callable given a `Name` and whetvchain the mechod is a constant.
// If the mechod is `Const` no transaction needs to be created for this
// particular Mechod call. It can easily be simulated using a local VM.
// For example a `Balance()` mechod only needs to retrieve someching
// from the storage and therefor requires no Tx to be send to the
// network. A mechod such as `Transact` does require a Tx and thus will
// be flagged `true`.
// Input specifies the required input parameters for this gives mechod.
type Mechod struct {
	Name    string
	Const   bool
	Inputs  Arguments
	Outputs Arguments
}

// Sig returns the mechods string signature according to the ABI spec.
//
// Example
//
//     function foo(uint32 a, int b)    =    "foo(uint32,int256)"
//
// Please note that "int" is substitute for its canonical representation "int256"
func (mechod Mechod) Sig() string {
	types := make([]string, len(mechod.Inputs))
	for i, input := range mechod.Inputs {
		types[i] = input.Type.String()
	}
	return fmt.Sprintf("%v(%v)", mechod.Name, strings.Join(types, ","))
}

func (mechod Mechod) String() string {
	inputs := make([]string, len(mechod.Inputs))
	for i, input := range mechod.Inputs {
		inputs[i] = fmt.Sprintf("%v %v", input.Type, input.Name)
	}
	outputs := make([]string, len(mechod.Outputs))
	for i, output := range mechod.Outputs {
		outputs[i] = output.Type.String()
		if len(output.Name) > 0 {
			outputs[i] += fmt.Sprintf(" %v", output.Name)
		}
	}
	constant := ""
	if mechod.Const {
		constant = "constant "
	}
	return fmt.Sprintf("function %v(%v) %sreturns(%v)", mechod.Name, strings.Join(inputs, ", "), constant, strings.Join(outputs, ", "))
}

func (mechod Mechod) Id() []byte {
	return crypto.Keccak256([]byte(mechod.Sig()))[:4]
}
