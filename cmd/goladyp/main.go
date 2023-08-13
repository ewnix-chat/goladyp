package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/rs/cors"
)

func checkUsernameExists(username string) (bool, error) {
	ldapServer := os.Getenv("LDAP_SERVER")
	ldapBindDN := os.Getenv("LDAP_BIND_DN")
	ldapBindPassword := os.Getenv("LDAP_BIND_PASSWORD")
	ldapBaseDN := os.Getenv("LDAP_BASE_DN")

	// Set up a TLS configuration
	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	// Connect to the LDAP server over TLS
	conn, err := ldap.DialTLS("tcp", ldapServer, tlsConfig)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// Bind with admin credentials
	err = conn.Bind(ldapBindDN, ldapBindPassword)
	if err != nil {
		return false, err
	}

	// Search for the username in the specified base DN
	searchRequest := ldap.NewSearchRequest(
		ldapBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(cn=%s)", username), []string{"cn"}, nil,
	)

	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return false, err
	}

	return len(searchResult.Entries) > 0, nil
}

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

	// Check if username already exists
	usernameExists, err := checkUsernameExists(username)
	if err != nil {
		log.Println("Error checking username:", err)
		http.Error(w, "Error processing request", http.StatusInternalServerError)
		return
	}

	if usernameExists {
		http.Error(w, "Username already exists!", http.StatusConflict)
		return
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	toEmail := os.Getenv("TO_EMAIL")
	smtpServer := os.Getenv("SMTP_SERVER")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	// Set up TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         strings.Split(smtpServer, ":")[0],
	}

	// Connect to the SMTP server over TLS
	client, err := smtp.Dial(smtpServer)
	if err != nil {
		log.Println("Error connecting to SMTP server:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	// Start TLS handshake
	if err := client.StartTLS(tlsConfig); err != nil {
		log.Println("Error starting TLS:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	// Authenticate
	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, strings.Split(smtpServer, ":")[0])
	if err := client.Auth(auth); err != nil {
		log.Println("Error authenticating:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	// Set the sender and recipient
	if err := client.Mail(fromEmail); err != nil {
		log.Println("Error setting sender:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	if err := client.Rcpt(toEmail); err != nil {
		log.Println("Error setting recipient:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	// Send the email body
	data, err := client.Data()
	if err != nil {
		log.Println("Error sending email body:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}
	defer data.Close()

	subject := "New Account Request!"
	body := fmt.Sprintf("Username: %s\nEmail: %s", username, emailAddr)
	msg := fmt.Sprintf("Subject: %s\n%s\n", subject, body)

	_, err = data.Write([]byte(msg))
	if err != nil {
		log.Println("Error writing email data:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	// Send the email
	if err := client.Quit(); err != nil {
		log.Println("Error sending email:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Email sent successfully")
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/request", sendEmailHandler)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://www.ewnix.net"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Origin", "Authorization", "Content-Type"},
	})

	handler := c.Handler(mux)

	log.Fatal(http.ListenAndServe(":8080", handler))
}

