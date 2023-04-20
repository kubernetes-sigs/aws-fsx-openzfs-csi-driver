/*
Copyright 2023 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"reflect"
	"testing"
)

func TestSplitCommasAndRemoveOuterBrackets(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "Split commas",
			input: "Value1,Key2=Value2,Key3=[Value3],Key4={Value4},[Value5],{Value6}",
			expected: []string{
				"Value1",
				"Key2=Value2",
				"Key3=[Value3]",
				"Key4={Value4}",
				"[Value5]",
				"Value6",
			},
		},
		{
			name:  "Split array",
			input: "[Value1,Key2=Value2,Key3=[Value3],Key4={Value4},[Value5],{Value6}]",
			expected: []string{
				"Value1",
				"Key2=Value2",
				"Key3=[Value3]",
				"Key4={Value4}",
				"[Value5]",
				"Value6",
			},
		},
		{
			name:  "Split object",
			input: "{Value1,Key2=Value2,Key3=[Value3],Key4={Value4},[Value5],{Value6}}",
			expected: []string{
				"Value1",
				"Key2=Value2",
				"Key3=[Value3]",
				"Key4={Value4}",
				"[Value5]",
				"Value6",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := SplitCommasAndRemoveOuterBrackets(tc.input)

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("SplitCommasAndRemoveOuterBrackets got wrong result. actual: %s, expected: %s", actual, tc.expected)
			}
		})
	}
}

func TestMapValues(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected map[string]*string
		error    bool
	}{
		{
			name: "Success",
			input: []string{
				"Key1=Value1",
				"Key2=[Value2]",
				"Key3={Value3}",
				"Key4=",
				"Key5=Value5=SubValue5",
			},
			expected: map[string]*string{
				"Key1": aws.String("Value1"),
				"Key2": aws.String("[Value2]"),
				"Key3": aws.String("{Value3}"),
				"Key4": nil,
				"Key5": aws.String("Value5=SubValue5"),
			},
			error: false,
		},
		{
			name: "Failure",
			input: []string{
				"Value1",
			},
			expected: map[string]*string{},
			error:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mappedValues, err := MapValues(tc.input)

			if err != nil && tc.error == false || err == nil && tc.error == true {
				t.Fatalf("MapValues got wrong result. Error mismatch.")
			}

			actual := aws.StringValueMap(mappedValues)
			expected := aws.StringValueMap(tc.expected)

			if fmt.Sprint(actual) != fmt.Sprint(expected) {
				t.Fatalf("MapValues got wrong result. actual: %s, expected: %s", fmt.Sprint(actual), fmt.Sprint(tc.expected))
			}
		})
	}
}
