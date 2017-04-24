package feed

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/lib"

	"github.com/streadway/amqp"
)

// local context
type fCtx struct {
	*lib.Ctx

	Producer *lib.QueueHandler // the queue read by check
}

// Run starts the feed module either blocking or non-blocking.
func Run(ctx *lib.Ctx, blocking bool) error {
	producer, err := ctx.SetupQueue("totem-dynamic-check-" + ctx.Config.QueueSuffix)
	if err != nil {
		return err
	}

	c := &fCtx{
		ctx,
		producer,
	}

	if blocking {
		c.Consume(ctx.Config.ConsumeQueue, ctx.Config.FeedPrefetchCount, c.parseMsg)
	} else {
		go c.Consume(ctx.Config.ConsumeQueue, ctx.Config.FeedPrefetchCount, c.parseMsg)
	}

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

	for serviceName, _ := range req.Tasks {
		urls, check := c.Config.Services[serviceName]
		if !check {
			//c.NackOnError(errors.New(serviceName+" not found"), "Service is not existing on this node", msg)
			//return
			c.Warning.Println("Service is not existing on this node")
			continue
		}

		if len(urls) == 0 {
			c.Warning.Println("Service is existing in config but no URLs are supplied")
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

// handleFeeding checks the status of the respective service
// and uploads the new sample if everything is fine. If not
// either an error is send or a waiting timer is actived.
func (c *fCtx) handleFeeding(req *lib.ExternalRequest, service *lib.Service, msg *amqp.Delivery) {
	// get the status of the service
	status, err := service.Status()
	if c.NackOnError(err, "Service is not existing on this node", msg) {
		return
	}

	// check if the service has free capacity
	for status.FreeSlots <= 0 {
		c.Debug.Println("Slowdown: No free slots")
		time.Sleep(time.Second * 30)

		status, err = service.Status()
		if c.NackOnError(err, "Service is not existing on this node", msg) {
			return
		}
	}

	// differentiate between downloadable samples and URLs
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

		sample = filepath.Base(tmpFile.Name())
	} else {
		// we do not need to download the sample
		// the filename "is the sample data"
		sample = req.Filename
	}

	// create new task
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

	// send to check
	c.Producer.Send(internalReq)
	if err := msg.Ack(false); err != nil {
		c.Warning.Println("Sending ACK failed!", err.Error())
	}
}
