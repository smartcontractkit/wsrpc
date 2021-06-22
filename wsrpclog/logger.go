package wsrpclog

import "log"

// Logger defines a simple logger interface. We may want to consider improving
// logging capabilities.
type Logger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

var (
	Default Logger
)

func init() {
	Default = &logger{}
}

func SetVerboseLogger() {
	Default = &logger{Verbose: true}
}

type logger struct {
	Verbose bool
}

var _ Logger = (*logger)(nil)

func (l *logger) Print(v ...interface{}) {
	if l.Verbose {
		log.Println(v...)
	}
}

func (l *logger) Printf(format string, v ...interface{}) {
	if l.Verbose {
		log.Printf(format, v...)
	}
}

func (l *logger) Println(v ...interface{}) {
	if l.Verbose {
		log.Println(v...)
	}
}

func Print(v ...interface{}) {
	Default.Print(v...)
}

func Printf(format string, v ...interface{}) {
	Default.Printf(format, v...)
}

func Println(v ...interface{}) {
	Default.Println(v...)
}
