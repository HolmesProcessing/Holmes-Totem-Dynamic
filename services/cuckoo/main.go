package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"cuckoo/cuckoo"
)

type Config struct {
	HTTPBinding    string
	VerifySSL      bool
	CheckFreeSpace bool
	CuckooURL      string
	MaxPending     int
	MaxAPICalls    int
	LogFile        string
	LogLevel       string
}

type Ctx struct {
	Config *Config
	Cuckoo *cuckoo.Cuckoo
}

type RespStatus struct {
	Degraded  bool
	Error     string
	FreeSlots int
}

type RespNewTask struct {
	Error  string
	TaskID string
}

type RespCheckTask struct {
	Error string
	Done  bool
}

type RespTaskResults struct {
	Error   string
	Results interface{}
}

// TODO: Replace this with our own schema es soon as we have one
type CrtResult struct {
	Subtype string
	Result  string
	Data    map[string]interface{}
}

var (
	ctx *Ctx
)

func main() {
	// prepare context
	ctx = &Ctx{
		Config: &Config{},
	}

	cFile, err := os.Open("./service.conf")
	if err != nil {
		panic(err.Error())
	}

	decoder := json.NewDecoder(cFile)
	err = decoder.Decode(ctx.Config)
	if err != nil {
		panic(err.Error())
	}

	cuckoo, err := cuckoo.New(ctx.Config.CuckooURL, ctx.Config.VerifySSL)
	if err != nil {
		panic(err.Error())
	}
	ctx.Cuckoo = cuckoo

	// prepare routing
	r := http.NewServeMux()
	r.HandleFunc("/status/", HTTPStatus)
	r.HandleFunc("/feed/", HTTPFeed)
	r.HandleFunc("/check/", HTTPCheck)
	r.HandleFunc("/results/", HTTPResults)

	srv := &http.Server{
		Handler:      r,
		Addr:         ctx.Config.HTTPBinding,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func HTTPStatus(w http.ResponseWriter, r *http.Request) {
	resp := &RespStatus{
		Degraded:  false,
		Error:     "",
		FreeSlots: 0,
	}

	s, err := ctx.Cuckoo.GetStatus()
	if err != nil {
		resp.Error = err.Error()
		HTTP500(w, r, resp)
		return
	}

	resp.FreeSlots = ctx.Config.MaxPending - s.Tasks.Pending

	if ctx.Config.CheckFreeSpace {
		if s.Diskspace != nil &&
			s.Diskspace.Analyses != nil &&
			s.Diskspace.Analyses.Free <= 256*1024*1024 {
			resp.Degraded = true
			resp.Error = "Disk is full!"
		}
	}

	json.NewEncoder(w).Encode(resp)
}

func HTTPFeed(w http.ResponseWriter, r *http.Request) {
	resp := &RespNewTask{
		Error:  "",
		TaskID: "",
	}

	sample := r.URL.Query().Get("obj")
	if sample == "" {
		resp.Error = "No sample given"
		HTTP500(w, r, resp)
		return
	}

	sampleBytes, err := ioutil.ReadFile("/tmp/" + sample)
	if err != nil {
		resp.Error = err.Error()
		HTTP500(w, r, resp)
		return
	}

	// TODO: actually fill payload, but therefore the payload
	// has to be send by totem-dyn in the first place.
	payload := make(map[string]string)

	taskID, err := ctx.Cuckoo.NewTask(sampleBytes, sample, payload)
	if err != nil {
		resp.Error = err.Error()
		HTTP500(w, r, resp)
		return
	}

	resp.TaskID = strconv.Itoa(taskID)

	json.NewEncoder(w).Encode(resp)
}

func HTTPCheck(w http.ResponseWriter, r *http.Request) {
	resp := &RespCheckTask{
		Error: "",
		Done:  false,
	}

	taskIDstr := r.URL.Query().Get("taskid")
	if taskIDstr == "" {
		resp.Error = "No taskID given"
		HTTP500(w, r, resp)
		return
	}
	taskID, _ := strconv.Atoi(taskIDstr)

	s, err := ctx.Cuckoo.TaskStatus(taskID)
	if err != nil {
		resp.Error = err.Error()
		HTTP500(w, r, resp)
		return
	}

	resp.Done = (s == "reported")

	json.NewEncoder(w).Encode(resp)
}

func HTTPResults(w http.ResponseWriter, r *http.Request) {
	resp := &RespTaskResults{
		Error: "",
	}

	taskIDstr := r.URL.Query().Get("taskid")
	if taskIDstr == "" {
		resp.Error = "No taskID given"
		HTTP500(w, r, resp)
		return
	}
	taskID, _ := strconv.Atoi(taskIDstr)

	///

	// get report
	report, err := ctx.Cuckoo.TaskReport(taskID)
	if err != nil {
		resp.Error = err.Error()
		HTTP500(w, r, resp)
		return
	}

	///

	// build result
	resStructs := []*CrtResult{}

	// info
	resStructs = processReportInfo(report.Info)

	// signatures
	resStructs = append(resStructs, processReportSignatures(report.Signatures)...)

	// behavior
	resStructs = append(resStructs, processReportBehavior(report.Behavior)...)

	// dropped files
	/*
		// support for dropped files will be added later
		dResStructs, err := processDropped(m, cuckoo, nil, false)
		//if c.NackOnError(err, "processDropped failed", msg) {
		//	return
		//}
		if err != nil {
			c.Warning.Println("processDropped () exited with", err, "after dropping", len(dResStructs))
		}
		resStructs = append(resStructs, dResStructs...)
	*/

	if err = ctx.Cuckoo.DeleteTask(taskID); err != nil {
		log.Println("Cleaning cuckoo up failed for task", strconv.Itoa(taskID), err.Error())
	}

	json.NewEncoder(w).Encode(resp)
}

func HTTP500(w http.ResponseWriter, r *http.Request, response interface{}) {
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(response)
	return
}
