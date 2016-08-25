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

type Status struct {
	Degraded  bool
	Error     string
	FreeSlots int
}

type NewTask struct {
	Error  string
	TaskID string
}

type CheckTask struct {
	Error string
	Done  bool
}

type TaskResults struct {
	Error   string
	Results interface{}
}

func (s *Service) Status() (*Status, error) {
	status := &Status{}
	_, httpStatus, err := FastGet(s.Client, s.URL+"/status/", status)
	if httpStatus != 200 && err == nil {
		err = errors.New("Returned non-200 status code")
	}

	return status, err
}

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
