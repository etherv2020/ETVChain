// Copyright 2016 The go-etvchaineum Authors
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
	"strings"
	"testing"
)

const mechoddata = `
[
	{ "type" : "function", "name" : "balance", "constant" : true },
	{ "type" : "function", "name" : "send", "constant" : false, "inputs" : [ { "name" : "amount", "type" : "uint256" } ] },
	{ "type" : "function", "name" : "transfer", "constant" : false, "inputs" : [ { "name" : "from", "type" : "address" }, { "name" : "to", "type" : "address" }, { "name" : "value", "type" : "uint256" } ], "outputs" : [ { "name" : "success", "type" : "bool" } ]  }
]`

func TestMechodString(t *testing.T) {
	var table = []struct {
		mechod      string
		expectation string
	}{
		{
			mechod:      "balance",
			expectation: "function balance() constant returns()",
		},
		{
			mechod:      "send",
			expectation: "function send(uint256 amount) returns()",
		},
		{
			mechod:      "transfer",
			expectation: "function transfer(address from, address to, uint256 value) returns(bool success)",
		},
	}

	abi, err := JSON(strings.NewReader(mechoddata))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range table {
		got := abi.Mechods[test.mechod].String()
		if got != test.expectation {
			t.Errorf("expected string to be %s, got %s", test.expectation, got)
		}
	}
}