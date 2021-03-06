/*
Copyright 2014 Google Inc. All rights reserved.

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

package errors

import (
	"fmt"
	"testing"
)

func TestMakeFuncs(t *testing.T) {
	testCases := []struct {
		fn       func() ValidationError
		expected ValidationErrorEnum
	}{
		{
			func() ValidationError { return NewInvalid("f", "v") },
			Invalid,
		},
		{
			func() ValidationError { return NewNotSupported("f", "v") },
			NotSupported,
		},
		{
			func() ValidationError { return NewDuplicate("f", "v") },
			Duplicate,
		},
		{
			func() ValidationError { return NewNotFound("f", "v") },
			NotFound,
		},
	}

	for _, testCase := range testCases {
		err := testCase.fn()
		if err.Type != testCase.expected {
			t.Errorf("expected Type %q, got %q", testCase.expected, err.Type)
		}
	}
}

func TestErrorList(t *testing.T) {
	errList := ErrorList{}
	errList = append(errList, NewInvalid("field", "value"))
	// The fact that this compiles is the test.
}

func TestErrorListToError(t *testing.T) {
	errList := ErrorList{}
	err := errList.ToError()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	testCases := []struct {
		errs     ErrorList
		expected string
	}{
		{ErrorList{fmt.Errorf("abc")}, "abc"},
		{ErrorList{fmt.Errorf("abc"), fmt.Errorf("123")}, "abc; 123"},
	}
	for _, testCase := range testCases {
		err := testCase.errs.ToError()
		if err == nil {
			t.Errorf("expected an error, got nil: ErrorList=%v", testCase)
			continue
		}
		if err.Error() != testCase.expected {
			t.Errorf("expected %q, got %q", testCase.expected, err.Error())
		}
	}
}
