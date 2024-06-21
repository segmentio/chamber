package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReservedService(t *testing.T) {
	assert.True(t, ReservedService(ChamberService))
	assert.False(t, ReservedService("not-reserved"))
}
