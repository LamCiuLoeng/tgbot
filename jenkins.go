package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Jenkins proxy object
type Jenkins struct {
	url       string
	user      string
	userToken string
	jobToken  string
}

// ExePart is the part neted in executable
type ExePart struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// QueueItem is the json form return from queue item url
type QueueItem struct {
	Executable ExePart                `json:"executable"`
	X          map[string]interface{} `json:"-"`
}

// BuildDetail is the json form return from build detail
type BuildDetail struct {
	Building bool                   `json:"building"`
	Result   string                 `json:"result"`
	X        map[string]interface{} `json:"-"`
}

// BasicInfo is the root json form return from root api
type BasicInfo struct {
	Jobs []map[string]string    `json:"jobs"`
	X    map[string]interface{} `json:"-"`
}

// SubmitJob is trigger the build of job
func (j Jenkins) SubmitJob(job, sha1 string) (string, error) {
	client := &http.Client{}
	var req *http.Request

	if sha1 != "" {
		apiURL := fmt.Sprintf("%s/job/%s/buildWithParameters?token=%s", j.url, job, j.jobToken)
		form := url.Values{}
		form.Add("sha1", sha1)
		body := strings.NewReader(form.Encode())
		req, _ = http.NewRequest("POST", apiURL, body)
	} else {
		apiURL := fmt.Sprintf("%s/job/%s/build?token=%s", j.url, job, j.jobToken)
		req, _ = http.NewRequest("POST", apiURL, nil)
	}

	req.SetBasicAuth(j.user, j.userToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	log.Println(resp.StatusCode)
	if resp.StatusCode != 201 {
		defer resp.Body.Close()
		respBody, _ := ioutil.ReadAll(resp.Body)
		log.Println(string(respBody))
		return "", fmt.Errorf("提交job[%s]失败", job)
	}
	location := resp.Header.Get("Location")
	u, _ := url.Parse(location)
	return u.Path, nil
}

// GetBuildNumber is to pull until getting the build number of the queue item
func (j Jenkins) GetBuildNumber(path string) (string, error) {
	apiURL := fmt.Sprintf("%s%s/api/json", j.url, path)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.SetBasicAuth(j.user, j.userToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	queueItem := QueueItem{}
	if err = json.Unmarshal(body, &queueItem); err != nil {
		return "", err
	}
	if queueItem.Executable.Number != 0 {
		return strconv.Itoa(queueItem.Executable.Number), nil
	}
	return "", fmt.Errorf("未能获取build number")
}

// CheckResult is to check whether the build is done and the result
func (j Jenkins) CheckResult(job, buildNumber string) (bool, bool, error) {
	apiURL := fmt.Sprintf("%s/job/%s/%s/api/json?token=%s", j.url, job, buildNumber, j.jobToken)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.SetBasicAuth(j.user, j.userToken)
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, false, err
	}
	buildDetail := BuildDetail{}
	if err = json.Unmarshal(body, &buildDetail); err != nil {
		return false, false, err
	}
	return buildDetail.Result == "SUCCESS", buildDetail.Building, nil
}

// GetLog is to pull the console log when the build is fail
func (j Jenkins) GetLog(job, buildNumber string) (string, error) {
	apiURL := fmt.Sprintf("%s/job/%s/%s/consoleText?token=%s", j.url, job, buildNumber, j.jobToken)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.SetBasicAuth(j.user, j.userToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	logBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	logRows := strings.Split(string(logBytes), "\n")
	var logRange []string
	if len(logRows) > 10 {
		logRange = logRows[len(logRows)-10:]
	} else {
		logRange = logRows
	}
	result := strings.Join(logRange, "\n")
	return result, nil
}

// ListJobs is to show all the jobs
func (j Jenkins) ListJobs() (jobNames []string, err error) {
	apiURL := fmt.Sprintf("%s/api/json", j.url)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.SetBasicAuth(j.user, j.userToken)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	baseInfo := BasicInfo{}
	json.Unmarshal(body, &baseInfo)
	for _, job := range baseInfo.Jobs {
		jobNames = append(jobNames, job["name"])
	}
	return
}

// NewJenkins is a Jenkins object facotry
func NewJenkins(url, user, userToken, jobToken string) Jenkins {
	return Jenkins{url: url, user: user, userToken: userToken, jobToken: jobToken}
}
