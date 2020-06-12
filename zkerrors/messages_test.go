package zkerrors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	assert.Equal(t, ZKErrCodeToMessage(ErrOk), "")
	assert.Equal(t, ZKErrCodeToMessage(errAPIError), "api error")
	assert.Equal(t, ZKErrCodeToMessage(9999), "unknown error")
}
