package check

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/lib"

	"github.com/streadway/amqp"
)

type cCtx struct {
	*lib.Ctx

	Producer *lib.QueueHandler // the queue read by submit
}

type watchElem struct {
	Req     *lib.InternalRequest
	Msg     *amqp.Delivery
	Service *lib.Service
}

var (
	watchMap      = make(map[string]*watchElem)
	watchMapMutex = &sync.Mutex{}
)

func Run(ctx *lib.Ctx) error {
	producer, err := ctx.SetupQueue("totem-dynamic-submit-" + ctx.Config.QueueSuffix)
	if err != nil {
		return err
	}

	c := &cCtx{
		ctx,
		producer,
	}

	go c.checkLoop()
	go c.Consume("totem-dynamic-check-"+ctx.Config.QueueSuffix, ctx.Config.CheckPrefetchCount, c.parseMsg)

	return nil
}

// parseMsg accepts an *amqp.Delivery and parses the body assuming
// it's a request from crits. On success the parsed struct is
// send to handleSubmit.
func (c *cCtx) parseMsg(msg amqp.Delivery) {
	req := &lib.InternalRequest{}
	err := json.Unmarshal(msg.Body, req)
	if c.NackOnError(err, "Could not decode json!", &msg) {
		return
	}

	// TODO: Validate msg
	//if c.NackOnError(m.Validate(), "Could not validate msg", &msg) {
	//	return
	//}

	watchMapMutex.Lock()
	watchMap[req.FilePath] = &watchElem{
		Req: req,
		Msg: &msg,
		Service: &lib.Service{
			Name:   req.Service,
			URL:    req.URL,
			Client: c.Client,
		},
	}
	watchMapMutex.Unlock()
}

// checkLoop loops over the watch map and checks if the
// task is done and if so sends the task to submit.
func (c *cCtx) checkLoop() {
	waitDuration := time.Second * time.Duration(c.Config.WaitBetweenRequests)

	for {
		time.Sleep(waitDuration) //This is here so an empty list does not result in full load

		for k, v := range watchMap {
			time.Sleep(waitDuration)

			// try to get task status
			check, err := v.Service.CheckTask(v.Req.TaskID)
			if c.NackOnError(err, "Couldn't get status of task!", v.Msg) {
				delete(watchMap, k)
				continue
			}

			// if an error occured, remove from map and nack
			if check.Error != "" {
				c.NackOnError(errors.New(check.Error), "Checking task returned an error!", v.Msg)
				delete(watchMap, k)
				continue
			}

			// if task is not done continue to next task
			if !check.Done {
				continue
			}

			// task is done, send it to submit
			internalReq, err := json.Marshal(v.Req)
			if c.NackOnError(err, "Could not create internalRequest!", v.Msg) {
				return
			}

			c.Producer.Send(internalReq)
			if err := v.Msg.Ack(false); err != nil {
				c.Warning.Println("Sending ACK failed!", err.Error())
			}

			delete(watchMap, k)
		}
	}
}
