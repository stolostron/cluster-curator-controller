package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckErrorNil(t *testing.T) {

	InitKlog()
	assert.NotPanics(t, func() { CheckError(nil) }, "No panic, when err is not present")
}

func TestCheckErrorNotNil(t *testing.T) {

	assert.Panics(t, func() { CheckError(errors.New("TeST")) }, "Panics when a err is received")
}

func TestLogErrorNil(t *testing.T) {

	assert.Nil(t, LogError(nil), "err nil, when no err message")
}

func TestLogErrorNotNil(t *testing.T) {

	assert.NotNil(t, LogError(errors.New("TeST")), "err nil, when no err message")
}

// TODO, replace all instances of klog.Warning that include an IF, this saves us 2x lines of code
func TestLogWarning(t *testing.T) {

	assert.NotPanics(t, func() { LogWarning(nil) }, "No panic, when logging warnings")
	assert.NotPanics(t, func() { LogWarning(errors.New("TeST")) }, "No panic, when logging warnings")
}

func TestPathSplitterFromEnv(t *testing.T) {

	_, _, err := PathSplitterFromEnv("")
	assert.NotNil(t, err, "err not nil, when empty path")

	_, _, err = PathSplitterFromEnv("value")
	assert.NotNil(t, err, "err not nil, when only one value")

	_, _, err = PathSplitterFromEnv("value/")
	assert.NotNil(t, err, "err not nil, when only one value with split present")

	_, _, err = PathSplitterFromEnv("/value")
	assert.NotNil(t, err, "err not nil, when only one value with split present")

	namespace, secretName, err := PathSplitterFromEnv("ns1/s1")

	assert.Nil(t, err, "err nil, when path is split successfully")
	assert.Equal(t, namespace, "ns1", "namespace should be ns1")
	assert.Equal(t, secretName, "s1", "secret name should be s1")

}
