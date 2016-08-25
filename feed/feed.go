package feed

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/lib"

	"github.com/streadway/amqp"
)

type fCtx struct {
	*lib.Ctx

	Producer *lib.QueueHandler // the queue read by check
}

func Run(ctx *lib.Ctx) error {
	producer, err := ctx.SetupQueue("totem-dynamic-check-" + ctx.Config.QueueSuffix)
	if err != nil {
		return err
	}

	c := &fCtx{
		ctx,
		producer,
	}

	go c.Consume(ctx.Config.ConsumeQueue, ctx.Config.FeedPrefetchCount, c.parseMsg)

	return nil
}

// parseMsg accepts an *amqp.Delivery and parses the body assuming
// it's a request from the gateway. On success the parsed struct is
// send to handleFeeding.
func (c *fCtx) parseMsg(msg amqp.Delivery) {
	req := &lib.ExternalRequest{}
	err := json.Unmarshal(msg.Body, req)
	if c.NackOnError(err, "Could not decode json!", &msg) {
		return
	}

	// TODO: Validate msg
	//if c.NackOnError(m.Validate(), "Could not validate msg", &msg) {
	//	return
	//}

	// TODO: revalidate if this is necessary at all...
	// spawn in a new thread since the request
	// via web can be time consuming and we don't
	// want to slow down the queue processing
	for serviceName, _ := range req.Tasks {
		urls, check := c.Config.Services[serviceName]
		if !check {
			//c.NackOnError(errors.New(serviceName+" not found"), "Service is not existing on this node", msg)
			//return
			c.Warning.Println("Service is not existing on this node")
			continue
		}

		service := &lib.Service{
			Name:   serviceName,
			URL:    urls[rand.Intn(len(urls))],
			Client: c.Client,
		}

		go c.handleFeeding(req, service, &msg)
	}
}

// handleSubmit schedules the upload of a new sample to
// cuckoo. On success the information is passed to the
// check_results queue.
func (c *fCtx) handleFeeding(req *lib.ExternalRequest, service *lib.Service, msg *amqp.Delivery) {
	// check if the service has open capacity
	status, err := service.Status()
	if c.NackOnError(err, "Service is not existing on this node", msg) {
		return
	}

	for status.FreeSlots <= 0 {
		c.Debug.Println("Slowdown: No free slots")
		time.Sleep(time.Second * 30)

		status, err = service.Status()
		if c.NackOnError(err, "Service is not existing on this node", msg) {
			return
		}
	}

	sample := ""
	if req.Download {
		// we need to download the sample to /tmp
		resp, err := c.Client.Get(req.PrimaryURI)
		if c.NackOnError(err, "Downloading the file failed from "+req.PrimaryURI+" failed", msg) {
			return
		}
		defer lib.SafeResponseClose(resp)

		// return if file does not exist
		if resp.StatusCode != 200 {
			c.NackOnError(errors.New(resp.Status), req.PrimaryURI+" returned non-200 status code", msg)
			return
		}

		fileBytes, err := ioutil.ReadAll(resp.Body)
		if c.NackOnError(err, "couldn't read downloaded file!", msg) {
			return
		}

		tmpFile, err := ioutil.TempFile("/tmp/", "totem-dynamic")
		if c.NackOnError(err, "couldn't create file in /tmp", msg) {
			return
		}

		err = ioutil.WriteFile(tmpFile.Name(), fileBytes, 0644)
		if c.NackOnError(err, "couldn't create file in /tmp", msg) {
			return
		}

		sample = tmpFile.Name()
	} else {
		// we do not need to download the sample
		// the filename "is the sample data"
		sample = req.Filename
	}

	resp, err := service.NewTask(sample)
	if c.NackOnError(err, "Feeding sample to service failed", msg) {
		return
	}

	internalReq, err := json.Marshal(lib.InternalRequest{
		Service:         service.Name,
		URL:             service.URL,
		TaskID:          resp.TaskID,
		FilePath:        sample,
		Started:         time.Now(),
		OriginalRequest: req,
	})
	if c.NackOnError(err, "Could not create internalRequest!", msg) {
		return
	}

	c.Producer.Send(internalReq)
	if err := msg.Ack(false); err != nil {
		c.Warning.Println("Sending ACK failed!", err.Error())
	}
}
