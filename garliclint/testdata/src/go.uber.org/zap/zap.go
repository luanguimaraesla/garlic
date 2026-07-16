package zap

type Field struct{}

type Logger struct{}

func Error(error) Field                { return Field{} }
func NewProduction() *Logger           { return &Logger{} }
func NewDevelopment() *Logger          { return &Logger{} }
func NewExample() *Logger              { return &Logger{} }
func NewNop() *Logger                  { return &Logger{} }
func (*Logger) Error(string, ...Field) {}
func (*Logger) Warn(string, ...Field)  {}
func (*Logger) Info(string, ...Field)  {}
func (*Logger) Debug(string, ...Field) {}
