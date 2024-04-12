package chainstatemigration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPercentage(t *testing.T) {
	res := percentage(10, 100)
	require.Equal(t, float64(10), res)
}

func TestPushElementToPlace(t *testing.T) {
	testCases := []struct {
		list          []interface{}
		expectedList  []interface{}
		element       interface{}
		placeTo       int
		expectedError error
	}{
		{
			list:          []any{1, 2, 3},
			expectedList:  nil,
			element:       0,
			placeTo:       -1,
			expectedError: ErrNoNegativePlaces,
		},
		{
			list:          []any{},
			expectedList:  []any{1},
			element:       1,
			placeTo:       0,
			expectedError: nil,
		},
		{
			list:          []any{1},
			expectedList:  []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1},
			element:       1,
			placeTo:       10,
			expectedError: nil,
		},
		{
			list:          []any{1, nil, 3},
			expectedList:  []any{1, 2, 3},
			element:       2,
			placeTo:       1,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		newList, err := PushElementToPlace(tc.list, tc.element, tc.placeTo)
		assert.ErrorIs(t, err, tc.expectedError)
		assert.Equal(t, tc.expectedList, newList)
	}
}
