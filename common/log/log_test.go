package log

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func simple() error {
	return fmt.Errorf("error from %s", "simple")
}

func foo() error {
	return errors.Wrap(simple(), "Error from foo")
}

func bar() error {
	return errors.Wrap(foo(), "Error from bar")
}

func getLen(msg string, args ...interface{}) int {
	list := append([]interface{}{msg}, args...)
	return len(list)
}

func TestLogging(t *testing.T) {
	// *LogFormat = "json"
	BindFlag()
	err := bar()
	Errore(err, "show error")
	// Error("show error with detail: ", withDetail(err))
	// With(zap.String("stacktrace", withDetail(err))).Error("
	// error3")
	// globalRaw.With(zap.Error(err)).Error("error4")
	// WithError(err).Error("error5")
	// globalRaw.Error()

	assert.Equal(t, getLen("hello", "1", "2"), 3)
}

func TestLogging2(t *testing.T) {
	_, logger := FromContext(context.Background())

	fn1 := func() {
		logger.Infof("good") // caller is here
	}
	fn2 := func() {
		logger.AddCallerSkip(1).Infof("good")
	}

	main := func() {
		fn1()
		fn2() // caller is here
	}

	main()
}

func Test_LogEveryN(t *testing.T) {
	_, logger := FromContext(context.Background())
	for i := 0; i < 10; i++ {
		logger.InfoEveryN(5, "good")
	}
}
