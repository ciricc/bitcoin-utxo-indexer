package chainstatemigration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPercentage(t *testing.T) {
	res := percentage(10, 100)
	require.Equal(t, float64(10), res)
}
