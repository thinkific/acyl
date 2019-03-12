package errors

import (
	"fmt"
	"testing"
)

func TestUserErrorDetection(t *testing.T) {
	testCases := []struct {
		input          error
		expectedResult bool
	}{
		{
			input:          fmt.Errorf("not a user error"),
			expectedResult: false,
		},
		{
			input:          UserError("you did this"),
			expectedResult: true,
		},
	}
	for _, testCase := range testCases {
		outputResult := IsUserError(testCase.input)
		if outputResult != testCase.expectedResult {
			t.Fatalf("Incorrect user error detection, input (%v), expected: (%v)", testCase.input, testCase.expectedResult)
		}
	}
}
