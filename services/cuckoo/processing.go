package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"cuckoo/cuckoo"
)

// processReportInfo extracts all the data from the info
// section of the cuckoo report struct.
func processReportInfo(i *cuckoo.TasksReportInfo) []*CrtResult {
	if i == nil {
		return []*CrtResult{}
	}

	// i.machine can be string or struct so we
	// have to determin where to get our info
	machineString := ""
	err := json.Unmarshal(i.Machine, &machineString)
	if err != nil || machineString == "" {
		machineString = "FAILED"
	}

	mStruct := &cuckoo.TasksReportInfoMachine{}
	err = json.Unmarshal(i.Machine, mStruct)
	if err == nil && mStruct != nil {
		machineString = mStruct.Name
	}

	resMap := make(map[string]interface{})
	resMap["started"] = i.Started
	resMap["ended"] = i.Ended
	resMap["analysis_id"] = strconv.Itoa(i.Id)

	return []*CrtResult{&CrtResult{"info", machineString, resMap}}
}

// processReportSignatures extracts all the data from the signatures
// section of the cuckoo report struct.
func processReportSignatures(sigs []*cuckoo.TasksReportSignature) []*CrtResult {
	if sigs == nil {
		return []*CrtResult{}
	}

	l := len(sigs)
	res := make([]*CrtResult, l, l)
	resMap := make(map[string]interface{})

	for k, sig := range sigs {
		resMap["severity"] = strconv.Itoa(sig.Severity)
		resMap["name"] = sig.Name

		res[k] = &CrtResult{
			"signature",
			sig.Description,
			resMap,
		}
	}

	return res
}

// processReportBehavior extracts all the data from the behavior
// section of the cuckoo report struct.
func processReportBehavior(behavior *cuckoo.TasksReportBehavior) []*CrtResult {
	if behavior == nil {
		return []*CrtResult{}
	}

	var res []*CrtResult
	resMap := make(map[string]interface{})

	if behavior.Processes != nil {
		for _, p := range behavior.Processes {
			resMap["process_id"] = strconv.Itoa(p.Id)
			resMap["parent_id"] = strconv.Itoa(p.ParentId)
			resMap["first_seen"] = p.FirstSeen

			res = append(res, &CrtResult{
				"process",
				p.Name,
				resMap,
			})
		}
	}

	// push api calls
	// not mixed in with upper loop so we can make it optional later
	if behavior.Processes != nil {
		pushCounter := 0

		for _, p := range behavior.Processes {

			procDescription := fmt.Sprintf("%s (%d)", p.Name, p.Id)
			for _, c := range p.Calls {

				if pushCounter >= ctx.Config.MaxAPICalls {
					break
				}

				resMap := make(map[string]interface{})
				resMap["category"] = c.Category
				resMap["status"] = c.Status
				resMap["return"] = c.Return
				resMap["timestamp"] = c.Timestamp
				resMap["thread_id"] = c.ThreadId
				resMap["repeated"] = c.Repeated
				resMap["api"] = c.Api
				resMap["id"] = c.Id
				resMap["process"] = procDescription
				resMap["arguments"] = c.Arguments

				res = append(res, &CrtResult{
					"api_call",
					c.Api,
					resMap,
				})
				pushCounter += 1
			}
		}
	}

	if behavior.Summary != nil {
		if behavior.Summary.Files != nil {
			for _, b := range behavior.Summary.Files {
				res = append(res, &CrtResult{
					"file",
					b,
					nil,
				})
			}
		}

		if behavior.Summary.Keys != nil {
			for _, b := range behavior.Summary.Keys {
				res = append(res, &CrtResult{
					"registry_key",
					b,
					nil,
				})
			}
		}

		if behavior.Summary.Mutexes != nil {
			for _, b := range behavior.Summary.Mutexes {
				res = append(res, &CrtResult{
					"mutex",
					b,
					nil,
				})
			}
		}
	}

	return res
}

/*
// support for dropped files will be added later
func processDropped(m *lib.CheckResultsReq, cuckoo *lib.CuckooConn, crits *lib.CritsConn, upload bool) ([]*CrtResult, error) {
	start := time.Now()

	resp, err := cuckoo.GetDropped(m.TaskId)
	if err != nil {
		return []*CrtResult{}, err
	}

	results := []*CrtResult{}

	respReader := bytes.NewReader(resp)
	unbzip2 := bzip2.NewReader(respReader)
	untar := tar.NewReader(unbzip2)

	for {
		hdr, err := untar.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}

		if err != nil {
			return results, err
		}

		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			// no real file, might be a dir or symlink
			continue
		}

		name := filepath.Base(hdr.Name)
		fileData, err := ioutil.ReadAll(untar)

		if upload {

			id, err := crits.NewSample(fileData, name)

			// we need to add a short sleep here so tastypie won't crash.
			// this is a very ugly work around but sadly necessary
			time.Sleep(time.Second * 1)

			if err != nil {
				if err.Error() == "empty file" {
					continue
				}

				return results, err
			}

			if err = crits.ForgeRelationship(id); err != nil {
				return results, err
			}

			// see comment above
			time.Sleep(time.Second * 1)
		}

		resMap := make(map[string]interface{})
		resMap["md5"] = fmt.Sprintf("%x", md5.Sum(fileData))

		results = append(results, &CrtResult{
			"file_added",
			name,
			resMap,
		})
	}

	elapsed := time.Since(start)
	c.Debug.Printf("Uploaded %d dropped files in %s [%s]\n", len(results), elapsed, m.CritsData.AnalysisId)

	return results, nil
}
*/
