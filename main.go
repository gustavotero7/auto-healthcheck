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
	Host               string    `yaml:"host"`
	ExpectedStatusCode int       `yaml:"expected_status_code"`
	NotificationsCount int       `yaml:"-"`
	LastNotification   time.Time `yaml:"-"`
	CurrentStatus      string    `yaml:"-"`
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
	PingInterval         int64             `yaml:"ping_interval"`
	NotificationInterval int64             `yaml:"notification_interval"`
	Targets              []Target          `yaml:"targets"`
	EmailNotification    EmailNotification `yaml:"email_notification"`
	MaxNotifications     int               `yaml:"max_notifications"`
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
	log.Println(c.MaxNotifications)
	run(c)
}

func run(c *Config) {
	for true {
		for i, _ := range c.Targets {
			// Interval between targets ping
			t := &c.Targets[i]
			time.Sleep(time.Second)
			if err := healthCheck(t); err != nil {
				if time.Now().Unix() > (t.LastNotification.Unix()+(c.NotificationInterval*60)) && t.NotificationsCount < c.MaxNotifications {
					subject := "Healthcheck fail: " + t.Host
					if t.CurrentStatus != "" {
						subject = "REMINDER: " + subject
					}
					sendNotifications(c.EmailNotification, subject, err.Error())
					t.LastNotification = time.Now()
					t.NotificationsCount++
				}
				t.CurrentStatus = err.Error()
			} else {
				if t.CurrentStatus != "" {
					sendNotifications(c.EmailNotification, fmt.Sprintf("Back to normal: %s", t.Host), fmt.Sprintf("All OK: %s back to normal :)", t.Host))
					t.NotificationsCount = 0
					t.CurrentStatus = ""
				}
			}
		}
		// Interval between pings cicle
		time.Sleep(time.Second * time.Duration(c.PingInterval))
	}
}

func healthCheck(t *Target) error {
	r, err := ping(t.Host)
	if err != nil {
		log.Printf("ERROR: Ping failed for target %s due %s\n", t.Host, err)
		return fmt.Errorf("ERROR: Ping failed for target %s due %s", t.Host, err)
	}

	if r != nil {
		if r.StatusCode != t.ExpectedStatusCode {
			log.Printf("ERROR: Status Code (%d) is not expected(%d)\n", r.StatusCode, t.ExpectedStatusCode)
			return fmt.Errorf("ERROR: Status Code (%d) is not expected(%d)", r.StatusCode, t.ExpectedStatusCode)
		}
	} else {
		log.Printf("Got null response from target: %s\n", t.Host)
		return fmt.Errorf("Got null response from target: %s", t.Host)
	}
	log.Printf("OK: Ping target %s success\n", t.Host)
	return nil
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

func sendNotifications(en EmailNotification, status string, message string) {

	for _, to := range en.To {
		msg := "From: " + en.From.User + "\n" +
			"To: " + to + "\n" +
			"Subject: " + status + " \n\n" +
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
