package actioncable

import "fmt"

type testLogger struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
}

var _ Logger = (*testLogger)(nil)

func (l *testLogger) Debug(msg string) {
	l.debugMessages = append(l.debugMessages, msg)
}

func (l *testLogger) Info(msg string) {
	l.debugMessages = append(l.infoMessages, msg)
}

func (l *testLogger) Error(msg string) {
	l.errorMessages = append(l.errorMessages, msg)
}

func getCurrentTestLogger() *testLogger {
	tl, ok := logger.(*testLogger)

	if !ok {
		panic(fmt.Sprintf("%v is not a testLogger", logger))
	}

	return tl
}
