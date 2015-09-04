package main
import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus"
)


type MyJSONFormatter struct {
}

func init() {
}

func (f *MyJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Note this doesn't include Time, Level and Message which are available on
	// the Entry. Consult `godoc` on information about those fields or read the
	// source of the official loggers.
	entry.Data["new"] = 1
	entry.Data["msg"] = entry.Message
	serialized, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}

func main(){
	log.Println("Hello there")
	log.SetFormatter(new(MyJSONFormatter))
	log.Println("Hello there")
}