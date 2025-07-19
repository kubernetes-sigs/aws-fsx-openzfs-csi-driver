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
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

const (
	GiB = 1024 * 1024 * 1024
)

func ParseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tcp":
	case "unix":
		addr = path.Join("/", addr)
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, addr, nil
}

func BytesToGiB(volumeSizeGiB int64) int32 {
	return int32(volumeSizeGiB / GiB)
}

func GiBToBytes(volumeSizeGiB int32) int64 {
	return int64(volumeSizeGiB) * GiB
}

func Contains(slice []string, element string) bool {
	for _, value := range slice {
		if value == element {
			return true
		}
	}
	return false
}

func EncodeDeletionTag(input string) string {
	input = strings.ReplaceAll(input, "\"", " @ ")
	input = strings.ReplaceAll(input, ",", " . ")
	input = strings.ReplaceAll(input, "[", " - ")
	input = strings.ReplaceAll(input, "]", " _ ")
	input = strings.ReplaceAll(input, "{", " + ")
	input = strings.ReplaceAll(input, "}", " = ")
	return input
}

func DecodeDeletionTag(input string) string {
	input = strings.ReplaceAll(input, " @ ", "\"")
	input = strings.ReplaceAll(input, " . ", ",")
	input = strings.ReplaceAll(input, " - ", "[")
	input = strings.ReplaceAll(input, " _ ", "]")
	input = strings.ReplaceAll(input, " + ", "{")
	input = strings.ReplaceAll(input, " = ", "}")
	return input
}

// ConvertStringMapToAny converts a given map to map containing any values which are parsable by json.
// Strings should already be formatted as jsons, therefore the raw value is taken
// ints and bools are converted to their respective types for proper json parsing
func ConvertStringMapToAny(input map[string]string) map[string]any {
	output := make(map[string]any)
	for key, value := range input {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			output[key] = intVal
			continue
		}

		if boolVal, err := strconv.ParseBool(value); err == nil {
			output[key] = boolVal
			continue
		}

		output[key] = json.RawMessage(value)
	}
	return output
}

// ConvertObjectToJsonString converts a given object to a json string.
func ConvertObjectToJsonString(input any) (string, error) {
	bytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// ConvertJsonStringToObject converts a given input into an object.
// The input should be a formatted JSON string.
// The object must be passed in by reference
func ConvertJsonStringToObject(input string, object any) error {
	if input == "" {
		return nil
	}

	err := json.Unmarshal(json.RawMessage(input), object)
	if err != nil {
		return err
	}
	return nil
}

// ConvertObjectType converts a given object into another object type.
// This is done by converting the object into a JSON string, then using the
// JSON string to populate the parameters of a new object.
// The "to" object must be passed in by reference
func ConvertObjectType(from any, to any) error {
	fromString, err := ConvertObjectToJsonString(from)
	if err != nil {
		return err
	}

	err = ConvertJsonStringToObject(fromString, to)
	if err != nil {
		return err
	}
	return nil
}

// RemoveParametersAndPopulateObject uses a given map[string]string to populate a given pointer to a struct.
// Values used in the map are removed once used.
func RemoveParametersAndPopulateObject(parameters map[string]string, config any) error {
	from := ConvertStringMapToAny(parameters)
	err := ConvertObjectType(from, config)
	if err != nil && strings.Contains(err.Error(), "error calling MarshalJSON") {
		return identifyImproperlyFormattedParameter(from)
	}

	for i := 0; i < reflect.TypeOf(config).Elem().NumField(); i++ {
		name := reflect.TypeOf(config).Elem().Field(i).Name
		_, ok := parameters[name]
		if ok {
			delete(parameters, name)
		}
	}

	return err
}

// StrictRemoveParametersAndPopulateObject removes config variables from parameters, and replaces them with one combined json string.
// Also ensures that all parameters has been used
func StrictRemoveParametersAndPopulateObject(parameters map[string]string, config any) error {
	err := RemoveParametersAndPopulateObject(parameters, config)
	if err != nil {
		return err
	}

	if len(parameters) != 0 {
		return errors.New(fmt.Sprintf("expected no unknown fields, instead had the following: %s", parameters))
	}

	return nil
}

// ReplaceParametersAndPopulateObject removes config variables from parameters, and replaces them with one combined json string.
func ReplaceParametersAndPopulateObject(newKey string, parameters map[string]string, config any) error {
	err := RemoveParametersAndPopulateObject(parameters, config)
	if err != nil {
		return err
	}

	configJsonString, err := ConvertObjectToJsonString(config)
	if err != nil {
		return err
	}

	parameters[newKey] = configJsonString
	return nil
}

// MapCopy copies map onto new map to allow for element manipulation
// Used for testing
func MapCopy(oldMap map[string]string) map[string]string {
	newMap := make(map[string]string, len(oldMap))
	for key, value := range oldMap {
		newMap[key] = value
	}
	return newMap
}

// identifyImproperlyFormattedParameter is called if the JSON marshal fails to parse the parameter map.
// It attempts to convert each individual parameter into a JSON string to identify the specific
// parameter that is improperly formatted on the storage class. This allows us to provide a more
// granular error message to the user.
func identifyImproperlyFormattedParameter(parameters map[string]any) error {
	for key, value := range parameters {
		parameter := map[string]any{
			key: value,
		}
		_, err := ConvertObjectToJsonString(parameter)
		if err != nil {
			return fmt.Errorf("failed to parse parameter key=%s, value=%s with error=%s", key, value, err)
		}
	}
	return fmt.Errorf("failed to parse parameter JSON, but could not determine which parameter could not be parsed")
}
