package submit

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/lib"

	"github.com/streadway/amqp"
)

type sCtx struct {
	*lib.Ctx

	Producer *lib.QueueHandler // the queue read by submit
}

type Result struct {
	Filename         string    `json:"filename"`
	Data             string    `json:"data"`
	MD5              string    `json:"md5"`
	SHA1             string    `json:"sha1"`
	SHA256           string    `json:"sha256"`
	ServiceName      string    `json:"service_name"`
	Tags             []string  `json:"tags"`
	Comment          string    `json:"comment"`
	StartedDateTime  time.Time `json:"started_date_time"`
	FinishedDateTime time.Time `json:"finished_date_time"`
}

// Run starts the submit module either blocking or non-blocking.
func Run(ctx *lib.Ctx, blocking bool) error {
	producer, err := ctx.SetupQueue(ctx.Config.ResultsQueue) // should be "totem_output"
	if err != nil {
		return err
	}

	c := &sCtx{
		ctx,
		producer,
	}

	if blocking {
		c.Consume("totem-dynamic-submit-"+ctx.Config.QueueSuffix, ctx.Config.SubmitPrefetchCount, c.parseMsg)
	} else {
		go c.Consume("totem-dynamic-submit-"+ctx.Config.QueueSuffix, ctx.Config.SubmitPrefetchCount, c.parseMsg)
	}

	return nil
}

// parseMsg accepts an *amqp.Delivery and parses the body assuming
// it's a request from crits. On success the parsed struct is
// send to handleSubmit.
func (c *sCtx) parseMsg(msg amqp.Delivery) {
	req := &lib.InternalRequest{}
	err := json.Unmarshal(msg.Body, req)
	if c.NackOnError(err, "Could not decode json!", &msg) {
		return
	}

	// TODO: Validate msg
	//if c.NackOnError(m.Validate(), "Could not validate msg", &msg) {
	//	return
	//}

	go c.submitResults(req, &msg)
}

func (c *sCtx) submitResults(req *lib.InternalRequest, msg *amqp.Delivery) {
	service := &lib.Service{
		Name:   req.Service,
		URL:    req.URL,
		Client: c.Client,
	}

	serviceResults, err := service.TaskResults(req.TaskID)
	if c.NackOnError(err, "Could not get results", msg) {
		return
	}

	// TODO: check if it is still necessary to handle service results as string
	resultsJ, err := json.Marshal(serviceResults.Results)

	// generate the necessary hashes, differentiate between samples and urls
	var fileBytes []byte
	if req.OriginalRequest.Download {
		fileBytes, err = ioutil.ReadFile("/tmp/" + req.FilePath)
		if c.NackOnError(err, "Could not read sample file", msg) {
			return
		}
	} else {
		fileBytes = []byte("/tmp/" + req.FilePath)
	}

	hSHA256 := sha256.New()
	hSHA256.Write(fileBytes)
	sha256String := fmt.Sprintf("%x", hSHA256.Sum(nil))

	hSHA1 := sha1.New()
	hSHA1.Write(fileBytes)
	sha1String := fmt.Sprintf("%x", hSHA1.Sum(nil))

	hMD5 := md5.New()
	hMD5.Write(fileBytes)
	md5String := fmt.Sprintf("%x", hMD5.Sum(nil))

	// build the final result obj

	resultMsg, err := json.Marshal(Result{
		Filename:         req.OriginalRequest.Filename,
		Data:             string(resultsJ),
		MD5:              md5String,
		SHA1:             sha1String,
		SHA256:           sha256String,
		ServiceName:      req.Service,
		Tags:             req.OriginalRequest.Tags,
		Comment:          req.OriginalRequest.Comment,
		StartedDateTime:  req.Started,
		FinishedDateTime: time.Now(),
	})

	if c.NackOnError(err, "Could not marshal final result", msg) {
		return
	}

	c.Producer.Channel.Publish(
		"totem", // exchange
		req.Service+".result.static.totem", // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         resultMsg,
		},
	)

	if err := msg.Ack(false); err != nil {
		c.Warning.Println("Sending ACK failed!", err.Error())
	}

	// cleanup time
	if err := os.Remove("/tmp/" + req.FilePath); err != nil {
		c.Warning.Printf("Could not delete file %s: %s\n", req.FilePath, err.Error())
	}
}
