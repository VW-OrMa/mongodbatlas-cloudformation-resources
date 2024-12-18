package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	ActionEnabled  = "enabled"
	ActionCreated  = "created"
	ActionRelease  = "release"
	ActionDeleted  = "deleted"
	ActionDisabled = "disabled"
)

type Confirmation struct {
	Url      string            `json:"url"`
	Deadline time.Time         `json:"deadline"`
	Header   map[string]string `json:"header"`
	Method   string            `json:"method"`
}

func (c Confirmation) Request() (*http.Request, error) {
	r, err := http.NewRequest(c.Method, c.Url, nil)
	if err != nil {
		return nil, err
	}
	for name, value := range c.Header {
		r.Header.Add(name, value)
	}
	return r, nil
}

type Regions []string

type Notification struct {
	Action       string       `json:"action"`
	Version      int          `json:"version"`
	AwsAccountId string       `json:"aws_account_id"`
	AccountId    string       `json:"account_guid"`
	ProjectId    string       `json:"project_guid"`
	Confirmation Confirmation `json:"confirmation"`
	Regions      Regions      `json:"regions"`
	ProjectEmail string       `json:"project_email"`
}

func ReadNotification(event string) (Notification, error) {
	var n Notification

	err := json.Unmarshal(json.RawMessage(event), &n)
	if err != nil {
		return Notification{}, fmt.Errorf("unmarshal details: %w", err)
	}

	return n, nil
}
