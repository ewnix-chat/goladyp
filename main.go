package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
)

func sendEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	emailAddr := r.FormValue("email")

	if username == "" || emailAddr == "" {
		http.Error(w, "Username and email are required", http.StatusBadRequest)
		return
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	toEmail := os.Getenv("TO_EMAIL")
	smtpServer := os.Getenv("SMTP_SERVER")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	// Set up TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Only use this in development/testing
		ServerName:         smtpServer,
	}

	client, err := smtp.Dial(smtpServer + ":465")
	if err != nil {
		log.Println("Error connecting to SMTP server:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	if err = client.StartTLS(tlsConfig); err != nil {
		log.Println("Error starting TLS:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpServer)

	msg := []byte("To: " + toEmail + "\r\n" +
		"Subject: New Account Request!\r\n" +
		"\r\n" +
		fmt.Sprintf("Username: %s\r\nE-mail: %s", username, emailAddr))

	if err := client.Auth(auth); err != nil {
		log.Println("Error authenticating:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	if err := client.Mail(fromEmail); err != nil {
		log.Println("Error sending mail from:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	if err := client.Rcpt(toEmail); err != nil {
		log.Println("Error sending mail to:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	writer, err := client.Data()
	if err != nil {
		log.Println("Error writing data:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	_, err = writer.Write(msg)
	if err != nil {
		log.Println("Error writing message:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	err = writer.Close()
	if err != nil {
		log.Println("Error closing writer:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Email sent successfully")
}

func main() {
	http.HandleFunc("/send-email", sendEmailHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

