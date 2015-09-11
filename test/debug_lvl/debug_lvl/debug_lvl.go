package debug_lvl
import (
	"fmt"
	"bytes"
	"github.com/Sirupsen/logrus"
	"os"
)

var DebugVisible = 2
var DebugLog = &logrus.Logger{
	Out: os.Stdout,
	Formatter: &DebugLvl{},
	Hooks: make(logrus.LevelHooks),
	Level: logrus.InfoLevel}

func Println(lvl int, args ...interface{}) {
	if lvl <= DebugVisible {
		DebugLog.WithField("debug_lvl", lvl).Println(args)
	}
}

type DebugLvl struct {
}

func (f *DebugLvl) Format(entry *logrus.Entry) ([]byte, error) {
	lvl := entry.Data["debug_lvl"].(int)
	if lvl <= DebugVisible {
		b := &bytes.Buffer{}
		b.WriteString(fmt.Sprintf("%d: %s", lvl, entry.Message))
		b.WriteByte('\n')

		return b.Bytes(), nil
	} else {
		return nil, nil
	}
}

