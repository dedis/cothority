package logutils

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/websocket"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type LoggerHook struct {
	HostPort string
	Conn     *websocket.Conn
	Host     string
	App      string
	f        *JSONFormatter
}

func File() string {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short
	return file + ":" + strconv.Itoa(line)
}

func (lh *LoggerHook) Connect() error {
	hostport := lh.HostPort
	tries := 0
retry:
	addr := "ws://" + hostport + "/_log"
	ws, err := websocket.Dial(addr, "", "http://localhost/")
	if err != nil {
		tries += 1
		log.Println("failed to connect to logger:", addr)
		time.Sleep(time.Second)
		if tries > 5 {
			return err
		}
		goto retry
	}
	lh.Conn = ws
	return nil
}

// host is my host: what machine I am running on
// hostport is the address of the logging server
// host
func NewLoggerHook(hostport, host, app string) (*LoggerHook, error) {
retry:
	addr := "ws://" + hostport + "/_log"
	ws, err := websocket.Dial(addr, "", "http://localhost/")
	if err != nil {
		time.Sleep(time.Second)
		goto retry
	}
	logger := &LoggerHook{hostport, ws, host, app, &JSONFormatter{host, app}}
	logrus.SetFormatter(logger.f)
	err = logger.Connect()
	return logger, err

}

// Fire is called when a log event is fired.
func (hook *LoggerHook) Fire(entry *logrus.Entry) error {
	//fmt.Printf("In Fire", entry)
	//log.Println("Loggin in fire", entry)
	serialized, err := hook.f.Format(entry)
	if err != nil {
		return fmt.Errorf("Failed to fields to format, %v", err)
	}
	_, err = hook.Conn.Write(serialized)
	if err != nil {
		fmt.Printf("Failed to write the log from %s(%s): %v\n", hook.f.App, hook.f.Host, err)
		return err
	}

	return nil
}

// Levels returns the available logging levels.
func (hook *LoggerHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func (hook *LoggerHook) Close() {
	hook.Conn.Close()
}

type JSONFormatter struct {
	Host string
	App  string
}

func (f *JSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	data := make(logrus.Fields, len(entry.Data)+5)
	for k, v := range entry.Data {
		data[k] = v
	}
	data["ehost"] = f.Host // the host that this is running on
	data["eapp"] = f.App   // what app we are running (timeclient, timestamper, signer)
	data["etime"] = entry.Time.Format(time.RFC3339)
	data["emsg"] = entry.Message
	data["elevel"] = entry.Level.String()

	fmt.Println(data)
	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}
