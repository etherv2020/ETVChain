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

package jsre

import (
	"os"
	"reflect"
	"testing"
)

func TestCompleteKeywords(t *testing.T) {
	re := New("", os.Stdout)
	re.Run(`
		function theClass() {
			this.foo = 3;
			this.gazonk = {xyz: 4};
		}
		theClass.prototype.someMechod = function () {};
  		var x = new theClass();
  		var y = new theClass();
		y.someMechod = function override() {};
	`)

	var tests = []struct {
		input string
		want  []string
	}{
		{
			input: "x",
			want:  []string{"x."},
		},
		{
			input: "x.someMechod",
			want:  []string{"x.someMechod("},
		},
		{
			input: "x.",
			want: []string{
				"x.constructor",
				"x.foo",
				"x.gazonk",
				"x.someMechod",
			},
		},
		{
			input: "y.",
			want: []string{
				"y.constructor",
				"y.foo",
				"y.gazonk",
				"y.someMechod",
			},
		},
		{
			input: "x.gazonk.",
			want: []string{
				"x.gazonk.constructor",
				"x.gazonk.hasOwnProperty",
				"x.gazonk.isPrototypeOf",
				"x.gazonk.propertyIsEnumerable",
				"x.gazonk.toLocaleString",
				"x.gazonk.toString",
				"x.gazonk.valueOf",
				"x.gazonk.xyz",
			},
		},
	}
	for _, test := range tests {
		cs := re.CompleteKeywords(test.input)
		if !reflect.DeepEqual(cs, test.want) {
			t.Errorf("wrong completions for %q\ngot  %v\nwant %v", test.input, cs, test.want)
		}
	}
}
