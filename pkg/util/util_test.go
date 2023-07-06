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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const (
	FailureExpectedError        = "%s didn't throw an error when one was expected"
	FailureThrewUnexpectedError = "%s threw the following error when one wasn't expected: %s"
	FailureWrongResult          = "%s got wrong result. actual: %s, expected: %s"
)

func TestContains(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name: "Success: element in slice",
			slice: []string{
				"element1",
				"element2",
				"element3",
			},
			element:  "element3",
			expected: true,
		},
		{
			name: "Success: element not in slice",
			slice: []string{
				"element1",
				"element2",
				"element3",
			},
			element:  "element4",
			expected: false,
		},
		{
			name:     "Success: slice is empty",
			slice:    []string{},
			element:  "element4",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := Contains(tc.slice, tc.element)

			if actual != tc.expected {
				t.Fatalf("Contains got wrong result. actual: %v, expected: %v", actual, tc.expected)
			}
		})
	}
}

func TestEncodeAndDecodeDeletionTag(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Success: Array of Objects",
			input:    "[{\"Key1\":\"Test\",\"Key2\":2},{\"Key1\",4,true}]",
			expected: " -  +  @ Key1 @ : @ Test @  .  @ Key2 @ :2 =  .  +  @ Key1 @  . 4 . true =  _ ",
		},
		{
			name:     "Success: Characters Used in Translation ",
			input:    "[{Test_Case-.},{5+5=10}]",
			expected: " -  + Test_Case-. =  .  + 5+5=10 =  _ ",
		},
		{
			name:     "Success: Empty",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encodedInput := EncodeDeletionTag(tc.input)
			decodedInput := DecodeDeletionTag(encodedInput)

			if encodedInput != tc.expected {
				t.Fatalf("EncodeDeletionTag got wrong result. actual: %s, expected: %s", encodedInput, tc.expected)
			}

			if decodedInput != tc.input {
				t.Fatalf("DecodeDeletionTag got wrong result. actual: %s, expected: %s", decodedInput, tc.input)
			}
		})
	}
}

func TestConvertStringMapToAny(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]string
		expected any
	}{
		{
			name: "Success: Multiple Types as Strings",
			input: map[string]string{
				"stringValue": "\"value\"",
				"intValue":    "1",
				"boolValue":   "true",
				"sliceValue":  "[val1, val2]",
				"objectValue": "{\"Key1\": \"Val1\"}",
				"nilValue":    "",
			},
			expected: map[string]any{
				"stringValue": json.RawMessage("\"value\""),
				"intValue":    int64(1),
				"boolValue":   true,
				"sliceValue":  json.RawMessage("[val1, val2]"),
				"objectValue": json.RawMessage("{\"Key1\": \"Val1\"}"),
				"nilValue":    json.RawMessage(""),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := ConvertStringMapToAny(tc.input)

			if !reflect.DeepEqual(output, tc.expected) {
				t.Fatalf(FailureWrongResult, "ConvertObjectToJsonString", output, tc.expected)
			}
		})
	}
}

func TestConvertObjectToJsonString(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected string
		error    bool
	}{
		{
			name: "Success: Array of Maps",
			input: []map[string]any{
				{
					"Key0": int64(1),
					"Key1": json.RawMessage("\"val1\""),
				},
				{
					"Key2": true,
				},
			},
			expected: "[{\"Key0\":1,\"Key1\":\"val1\"},{\"Key2\":true}]",
		},
		{
			name: "Failure: Invalid Json String",
			input: map[string]any{
				"Key1": json.RawMessage("{"),
			},
			error: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := ConvertObjectToJsonString(tc.input)

			if err != nil && tc.error == false {
				t.Fatalf(FailureThrewUnexpectedError, "ConvertObjectToJsonString", err)
			}

			if err == nil && tc.error == true {
				t.Fatalf(FailureExpectedError, "ConvertObjectToJsonString")
			}

			if err == nil && actual != tc.expected {
				t.Fatalf(FailureWrongResult, "ConvertObjectToJsonString", actual, tc.expected)
			}
		})
	}
}

func TestConvertJsonStringToObject(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		object   any
		expected any
		error    bool
	}{
		{
			name:   "Success: Array of Maps",
			input:  "[{\"Key0\":1,\"Key1\":\"val1\"},{\"Key2\":true}]",
			object: []map[string]any{},
			expected: []map[string]any{
				{
					"Key0": float64(1),
					"Key1": "val1",
				},
				{
					"Key2": true,
				},
			},
		},
		{
			name:   "Failure: Invalid json",
			input:  "{",
			object: map[string]string{},
			error:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ConvertJsonStringToObject(tc.input, &tc.object)

			if err != nil && tc.error == false {
				t.Fatalf(FailureThrewUnexpectedError, "ConvertObjectToJsonString", err)
			}

			if err == nil && tc.error == true {
				t.Fatalf(FailureExpectedError, "ConvertObjectToJsonString")
			}

			if err == nil && fmt.Sprint(tc.object) != fmt.Sprint(tc.expected) {
				t.Fatalf(FailureWrongResult, "ConvertObjectToJsonString", tc.object, tc.expected)
			}
		})
	}
}

func TestIdentifyImproperlyFormattedParameter(t *testing.T) {
	testCases := []struct {
		name  string
		input map[string]string
		error string
	}{
		{
			name: "Success: Improperly formatted SubnetIds",
			input: map[string]string{
				"DeploymentType":            "\"SINGLE_AZ_1\"",
				"ThroughputCapacity":        "64",
				"SubnetIds":                 "[\"subnet-016affca9638a1e61\"",
				"SkipFinalBackupOnDeletion": "true",
			},
			error: "failed to parse parameter key=SubnetIds, value=[\"subnet-016affca9638a1e61\"",
		},
		{
			name: "Success: Unable to identify improperly formatted parameter",
			input: map[string]string{
				"DeploymentType":            "\"SINGLE_AZ_1\"",
				"ThroughputCapacity":        "64",
				"SubnetIds":                 "[\"subnet-016affca9638a1e61\"]",
				"SkipFinalBackupOnDeletion": "true",
			},
			error: "failed to parse parameter JSON, but could not determine which parameter could not be parsed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := identifyImproperlyFormattedParameter(ConvertStringMapToAny(tc.input))
			if err == nil {
				t.Fatalf(FailureExpectedError, "identifyImproperlyFormattedParameter")
			}

			if !strings.Contains(err.Error(), tc.error) {
				t.Fatalf(FailureWrongResult, "identifyImproperlyFormattedParameter", err.Error(), tc.error)
			}
		})
	}
}
