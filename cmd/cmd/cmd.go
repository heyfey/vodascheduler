package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/heyfey/vodascheduler/config"
	"github.com/urfave/cli/v2"
)

// TODO: Get url from service ip
var url = "http://localhost:" + config.Port + config.EntryPoint

func CreateJob(c *cli.Context) error {
	file := c.String("filename")
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	resp, err := httpPost(data)
	if err != nil {
		return err
	} else {
		fmt.Println(string(resp))
	}
	return nil
}

func DeleteJob(c *cli.Context) error {
	job := c.Args().Get(0)
	if job == "" {
		return (errors.New("Must specify job name"))
	}
	// allow delete multiple jobs at once
	for i := 0; i < c.Args().Len(); i++ {
		job = c.Args().Get(0)
		resp, err := httpDelete(job)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(string(resp))
		}
	}
	return nil
}

func GetJobs(c *cli.Context) error {
	resp, err := httpGet()
	if err != nil {
		return err
	} else {
		fmt.Println(string(resp))
	}
	return nil
}

func httpGet() ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func httpPost(data []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	return (sendReq(req))
}

func httpDelete(job string) ([]byte, error) {
	req, err := http.NewRequest("DELETE", url, strings.NewReader(job))
	if err != nil {
		return nil, err
	}
	return (sendReq(req))
}

func sendReq(req *http.Request) ([]byte, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
