package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

type EmailReqData struct{
	Emails []string 	`json:"emails" validate:"required,min=1,dive,email"`
	Subject string 		`json:"subject" validate:"required,gt=0"`
	Content string 		`json:"content" validate:"required,gt=0"`
}

type SenderInfo struct {
    Email string `json:"email" validate:"required,email"`
}

type Recipient struct{
		Email string 	`json:"email" validate:"required,min=1,dive,email"` 
	}	

type EmailPayload struct{
	Sender SenderInfo `json:"sender" validate:"required"`
	To []Recipient `json:"to" validate:"required"`
	Subject string `json:"subject" validate:"required,gt=0"`
	HtmlContent string `json:"htmlContent" validate:"required,gt=0"`
}

func SendEmail(cntxt context.Context , payload json.RawMessage) error {
	var emaildata EmailReqData 
	err := json.Unmarshal(payload , &emaildata)
	if err != nil {
		log.Printf("Failed to unmarshal payload: %v", err)
		return err
	}
	if err = validate.Struct(&emaildata); err != nil {
		log.Printf("error while processing email payload %v" , err)
		return err
	} 

	From := os.Getenv("FROM")
	API_KEY := os.Getenv("API_KEY")

	var recipients []Recipient

	for _ , email := range emaildata.Emails{
		recipients = append(recipients, Recipient{Email: email})
	}

	htmlcontent := "<html><body><h1>"+emaildata.Content+"</h1></body></html>"

	emailpayload := EmailPayload{
    Sender: SenderInfo{
        Email: From,
    },
    To:          recipients,
    Subject:     emaildata.Subject,
    HtmlContent: htmlcontent,
}
	client := &http.Client{
		Timeout: 10*time.Second,
	}
	emailbytes , err := json.Marshal(emailpayload)
	req , err := http.NewRequestWithContext(cntxt , http.MethodPost , "https://api.brevo.com/v3/smtp/email" , bytes.NewBuffer(emailbytes))
	if err != nil {
		log.Printf("Failed to  create request : %v", err)
		return err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", API_KEY)
	req.Header.Set("content-type", "application/json")

	rsp, err := client.Do(req)
	
	if err != nil {
		log.Printf("Failed to make request : %v", err)
		return err
	}
	
	defer rsp.Body.Close()

	if rsp.StatusCode >= 400 {
		log.Printf("brevo API returned bad status code: %d", rsp.StatusCode)
		return errors.New("brevo returned bad request")
	}

	return nil
}