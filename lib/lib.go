package lib

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/streadway/amqp"
)

type Ctx struct {
	Config *Config

	Debug   *log.Logger
	Info    *log.Logger
	Warning *log.Logger

	AmqpConn *amqp.Connection
	Client   *http.Client

	Failed *QueueHandler
}

type Config struct {
	Amqp         string
	QueueSuffix  string
	ConsumeQueue string
	ResultsQueue string
	FailedQueue  string

	LogFile   string
	LogLevel  string
	VerifySSL bool

	Services map[string][]string

	// stuff for feed
	FeedPrefetchCount int

	// stuff for check
	CheckPrefetchCount  int
	WaitBetweenRequests int

	// stuff for submit
	SubmitPrefetchCount int
}

// request from outside to totem-dynamic
type ExternalRequest struct {
	PrimaryURI   string              `json:"primaryURI"`
	SecondaryURI string              `json:"secondaryURI"`
	Filename     string              `json:"filename"`
	Tasks        map[string][]string `json:"tasks"`
	Tags         []string            `json:"tags"`
	Comment      string              `json:"comment"`
	Download     bool                `json:"download"`
	Source       string              `json:"source"`
	Attempts     int                 `json:"attempts"`
}

// request between feed/check/submit
type InternalRequest struct {
	Service         string
	URL             string
	TaskID          string
	FilePath        string
	Started         time.Time
	OriginalRequest *ExternalRequest
}

func (c *Ctx) Init(cPath string) error {
	var err error

	c.Config, err = loadConfig(cPath)
	if err != nil {
		return err
	}

	c.setupLogging()

	c.Info.Println("Connecting to amqp server...")
	c.AmqpConn, err = amqp.Dial(c.Config.Amqp)
	if err != nil {
		return err
	}

	c.Failed, err = c.SetupQueue(c.Config.FailedQueue)
	if err != nil {
		return err
	}

	c.setupClient()

	return nil
}

func loadConfig(cPath string) (*Config, error) {
	cPath = strings.TrimSpace(cPath)

	// no path given, try to search in the local directory
	if cPath == "" {
		cPath, _ = filepath.Abs(filepath.Dir(os.Args[0]))
		cPath += "/totem-dynamic.conf.json"
	}

	conf := &Config{}
	cFile, err := os.Open(cPath)
	if err != nil {
		return conf, err
	}

	decoder := json.NewDecoder(cFile)
	err = decoder.Decode(&conf)
	if err != nil {
		return conf, err
	}

	// validate the suffix
	if conf.QueueSuffix == "" {
		err = errors.New("Suffix is missing")
	}

	return conf, err
}

func (c *Ctx) setupLogging() error {
	// default: only log to stdout
	handler := io.MultiWriter(os.Stdout)

	if c.Config.LogFile != "" {
		// log to file
		if _, err := os.Stat(c.Config.LogFile); os.IsNotExist(err) {
			err := ioutil.WriteFile(c.Config.LogFile, []byte(""), 0600)
			if err != nil {
				return err
			}
		}

		f, err := os.OpenFile(c.Config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		handler = io.MultiWriter(f, os.Stdout)
	}

	// TODO: clean this mess up...
	empty := io.MultiWriter()
	if c.Config.LogLevel == "warning" {
		c.Warning = log.New(handler, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
		c.Info = log.New(empty, "INFO: ", log.Ldate|log.Ltime)
		c.Debug = log.New(empty, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	} else if c.Config.LogLevel == "info" {
		c.Warning = log.New(handler, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
		c.Info = log.New(handler, "INFO: ", log.Ldate|log.Ltime)
		c.Debug = log.New(empty, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		c.Warning = log.New(handler, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
		c.Info = log.New(handler, "INFO: ", log.Ldate|log.Ltime)
		c.Debug = log.New(handler, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	return nil
}

// setupClient populates the http client so we have one client
// which can keep the connections open so there is no need to
// start a new connection for each request.
func (c *Ctx) setupClient() {
	tr := &http.Transport{}
	if !c.Config.VerifySSL {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	c.Client = &http.Client{Transport: tr}
}

// FastGet is a wrapper for http.Get which returns only
// the important data from the request.
func FastGet(c *http.Client, url string, structPointer interface{}) ([]byte, int, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer SafeResponseClose(resp)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	if structPointer != nil {
		err = json.Unmarshal(respBody, structPointer)
	}

	return respBody, resp.StatusCode, err
}

func SafeResponseClose(r *http.Response) {
	if r == nil {
		return
	}

	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()
}
