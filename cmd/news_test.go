package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFridayHookMessage_Friday(t *testing.T) {
	msg := fridayHookMessage(time.Friday)
	assert.NotEmpty(t, msg)
	assert.Contains(t, msg, "Friday")
	assert.Contains(t, msg, "sap-devs news latest")
}

func TestFridayHookMessage_NotFriday(t *testing.T) {
	nonFridays := []time.Weekday{
		time.Sunday,
		time.Monday,
		time.Tuesday,
		time.Wednesday,
		time.Thursday,
		time.Saturday,
	}
	for _, day := range nonFridays {
		assert.Empty(t, fridayHookMessage(day), "expected empty string for %s", day)
	}
}
