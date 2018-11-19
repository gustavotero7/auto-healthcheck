package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type Target struct {
	Host               string `yaml:"host"`
	ExpectedStatusCode int    `yaml:"expected_status_code"`
	Failing            bool   `yaml:"-"`
}

type EmailSender struct {
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int64  `yaml:"smtp_port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

type EmailNotification struct {
	From EmailSender `yaml:"from"`
	To   []string    `yaml:"to"`
}

type Config struct {
	PingInterval      int64             `yaml:"ping_interval"`
	Targets           []Target          `yaml:"targets"`
	EmailNotification EmailNotification `yaml:"email_notification"`
}

func main() {
	b, err := ioutil.ReadFile("conf.yml")
	if err != nil {
		panic(err)
	}

	c := &Config{}
	if err := yaml.Unmarshal(b, c); err != nil {
		panic(err)
	}

	run(c)
}

var running = true

func run(c *Config) {
	for running {
		for _, t := range c.Targets {
			time.Sleep(time.Second)

			r, err := ping(t.Host)
			if err != nil {
				log.Printf("ERROR: Ping failed for target %s due %s\n", t.Host, err)
				sendNotifications(c.EmailNotification, fmt.Sprintf("ERROR: Ping failed for target %s due %s\n", t.Host, err))
				continue
			}

			if r != nil {
				if r.StatusCode != t.ExpectedStatusCode {
					log.Printf("ERROR: Status Code (%d) is not expected(%d)\n", r.StatusCode, t.ExpectedStatusCode)
					sendNotifications(c.EmailNotification, fmt.Sprintf("Got invalid status code from target: %s", t.Host))
					continue
				}
			} else {
				sendNotifications(c.EmailNotification, fmt.Sprintf("Got null response from target: %s", t.Host))
				continue
			}
			log.Printf("OK: Ping target %s success\n", t.Host)
		}
		time.Sleep(time.Second * time.Duration(c.PingInterval))
	}
}

func healthCheck() {

}

type Response struct {
	StatusCode int
	Body       string
}

func ping(target string) (*Response, error) {
	log.Println("Ping: ", target)
	resp, err := http.Get(target)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	r := &Response{
		StatusCode: resp.StatusCode,
		Body:       string(b),
	}
	return r, nil
}

func sendNotifications(en EmailNotification, message string) {
	for _, to := range en.To {
		msg := "From: " + en.From.User + "\n" +
			"To: " + to + "\n" +
			"Subject: Status Changed: \n\n" +
			message

		err := smtp.SendMail(fmt.Sprintf("%s:%d", en.From.SMTPHost, en.From.SMTPPort),
			smtp.PlainAuth("", en.From.User, en.From.Password, en.From.SMTPHost),
			en.From.User, []string{to}, []byte(msg))

		if err != nil {
			log.Printf("smtp error: %s", err)
			return
		}
	}

	log.Print("All notifications sent")
}
