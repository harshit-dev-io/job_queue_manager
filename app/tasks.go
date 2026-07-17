package main

import (
	"context"
	"encoding/json"
	"log"
	"net/smtp"
	"os"
)

type EmailData struct{
	Emails []string 	`json:"emails" validate:"required,min=1,dive,email"`
	Subject string 		`json:"subject" validate:"required,gt=0"`
	Content string 		`json:"content" validate:"required,gt=0"`
}

func SendEmail(cntxt context.Context , payload json.RawMessage) error {
	var emaildata EmailData 
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
	SmtpHost := os.Getenv("SMTPHOST")
	SmtpPort := os.Getenv("SMTPPORT")
	SmtpUser := os.Getenv("SMTPUSER")
	SmtpPass := os.Getenv("SMTPPASS")

	auth := smtp.PlainAuth("",SmtpUser , SmtpPass , SmtpHost)

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := "<html><body><p>"+emaildata.Content+"</p></body></html>"

	message := []byte("From: " + From +"\n" + "Subject: " + emaildata.Subject + "\n" + mime + body)
	err = smtp.SendMail(
		SmtpHost+":"+SmtpPort,
		auth,
		From,
		[]string(emaildata.Emails),
		[]byte(message),
	)

	if err != nil {
		log.Printf("error while processing email %v" , err)
		return err
	}


	return nil
}