package lib

import (
	"errors"
	"net/http"
)

type Service struct {
	Name   string
	URL    string
	Client *http.Client
}

// json return of status request
type Status struct {
	Degraded  bool
	Error     string
	FreeSlots int
}

// json return of feed request
type NewTask struct {
	Error  string
	TaskID string
}

// json return of check request
type CheckTask struct {
	Error string
	Done  bool
}

// json return of results request
type TaskResults struct {
	Error   string
	Results interface{}
}

// Status gets the current status of the service and returns it
// as a Status struct.
func (s *Service) Status() (*Status, error) {
	status := &Status{}
	_, httpStatus, err := FastGet(s.Client, s.URL+"/status/", status)
	if httpStatus != 200 && err == nil {
		err = errors.New("Returned non-200 status code")
	}

	return status, err
}

// NewTask sends a new task to the service and returns the result
// as a NewTask struct.
func (s *Service) NewTask(sample string) (*NewTask, error) {
	nt := &NewTask{}
	_, httpStatus, err := FastGet(s.Client, s.URL+"/feed/"+sample, nt)
	if httpStatus != 200 && err == nil {
		err = errors.New("Returned non-200 status code")
	}

	if nt.Error != "" {
		err = errors.New(nt.Error)
	}

	return nt, err
}

// CheckTask gets the current status of a task from the service and
// return the result as a CheckTask struct.
func (s *Service) CheckTask(taskID string) (*CheckTask, error) {
	ct := &CheckTask{}
	_, httpStatus, err := FastGet(s.Client, s.URL+"/check/"+taskID, ct)
	if httpStatus != 200 && err == nil {
		err = errors.New("Returned non-200 status code")
	}

	if ct.Error != "" {
		err = errors.New(ct.Error)
	}

	return ct, err
}

// TaskResults collects the results for a given task from the service
// and returns them as a TaskResults struct.
func (s *Service) TaskResults(taskID string) (*TaskResults, error) {
	tr := &TaskResults{}
	_, httpStatus, err := FastGet(s.Client, s.URL+"/results/"+taskID, tr)
	if httpStatus != 200 && err == nil {
		err = errors.New("Returned non-200 status code")
	}

	if tr.Error != "" {
		err = errors.New(tr.Error)
	}

	return tr, err
}
