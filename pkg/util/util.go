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
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	GiB                            = 1024 * 1024 * 1024
	DefaultVolumeStorageRequestGiB = 1
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

func SplitCommasAndRemoveOuterBrackets(input string) []string {
	if strings.HasPrefix(input, "[") {
		input = strings.Trim(input, "[]")
	} else if strings.HasPrefix(input, "{") {
		input = strings.Trim(input, "{}")
	}

	var leftCurly []int
	var leftSquare []int

	var slices []string
	lastComma := 0

	for i, c := range input {
		if "{" == string(c) {
			leftCurly = append(leftCurly, i)
		} else if "[" == string(c) {
			leftSquare = append(leftSquare, i)
		} else if "}" == string(c) {
			if len(leftCurly) > 0 {
				leftCurly = leftCurly[:len(leftCurly)-1]
			}
		} else if "]" == string(c) {
			if len(leftSquare) > 0 {
				leftSquare = leftSquare[:len(leftSquare)-1]
			}
		} else if "," == string(c) {
			if len(leftCurly) == 0 && len(leftSquare) == 0 {
				value := input[lastComma:i]
				slices = append(slices, value)
				lastComma = i + 1
			}
		}
	}

	slices = append(slices, input[lastComma:])

	var values []string
	for _, val := range slices {
		if strings.HasPrefix(val, "{") {
			val = strings.Trim(val, "{}")
		}
		values = append(values, val)
	}
	return values
}

func MapValues(input []string) (map[string]*string, error) {
	valueMap := make(map[string]*string)
	for _, configOption := range input {
		splitKeyValue := strings.SplitN(configOption, "=", 2)
		if len(splitKeyValue) == 2 {
			if splitKeyValue[1] == "" {
				valueMap[splitKeyValue[0]] = nil
			} else {
				valueMap[splitKeyValue[0]] = &splitKeyValue[1]
			}
		} else {
			return nil, errors.New("Input doesn't contain =")
		}
	}
	return valueMap, nil
}

func BytesToGiB(volumeSizeGiB int64) int64 {
	return volumeSizeGiB / GiB
}

func GiBToBytes(volumeSizeGiB int64) int64 {
	return volumeSizeGiB * GiB
}

func StringToIntPointer(input string) (*int64, error) {
	output, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func StringToBoolPointer(input string) (*bool, error) {
	output, err := strconv.ParseBool(input)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func BoolToStringPointer(input bool) *string {
	output := strconv.FormatBool(input)
	return &output
}

func Contains(slice []string, element string) bool {
	for _, value := range slice {
		if value == element {
			return true
		}
	}
	return false
}
