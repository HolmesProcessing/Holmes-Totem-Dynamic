package cuckoo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"time"
)

func (cko *CuckooConn) GetStatus() (*CkoStatus, error) {
	r := &CkoStatus{}
	resp, status, err := cko.C.FastGet(cko.URL+"/cuckoo/status", r)
	if err != nil || status != 200 {
		if resp != nil {
			err = errors.New(fmt.Sprintf("%s -> [%d] %s", err.Error(), status, resp))
		}

		return nil, err
	}

	return r, nil
}
