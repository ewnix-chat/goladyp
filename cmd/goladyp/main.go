package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/rs/cors"
)

type RequestData struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func checkUsernameExists(username string) (bool, error) {
	ldapServer := os.Getenv("LDAP_SERVER")
	ldapPort := os.Getenv("LDAP_PORT")
	ldapBindDN := os.Getenv("LDAP_BIND_DN")
	ldapBindPassword := os.Getenv("LDAP_BIND_PASSWORD")
	ldapBaseDN := os.Getenv("LDAP_BASE_DN")

	tlsConfig := &tls.Config{InsecureSkipVerify: false}

	conn, err := ldap.DialTLS("tcp", fmt.Sprintf("%s:%s", ldapServer, ldapPort), tlsConfig)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	err = conn.Bind(ldapBindDN, ldapBindPassword)
	if err != nil {
		return false, err
	}

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

	var requestData RequestData
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	usernameExists, err := checkUsernameExists(requestData.Username)
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
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         strings.Split(smtpServer, ":")[0],
	}

	dialer := &net.Dialer{
		Timeout:   35 * time.Second, // Set the timeout to 35 seconds
		KeepAlive: 0,                // Disable keep-alive
	}

	// Create a custom dialer with a timeout
	conn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%s", smtpServer, smtpPort))
	if err != nil {
		log.Println("Error connecting to SMTP server:", err)
		http.Error(w, "Error sending email, could not connect to SMTP server.", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, smtpServer)
	if err != nil {
		log.Println("Error creating SMTP client:", err)
		http.Error(w, "Error sending email, could not create SMTP client.", http.StatusInternalServerError)
		return
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		log.Println("Error starting TLS:", err)
		http.Error(w, "Error sending email, TLS error", http.StatusInternalServerError)
		return
	}

	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, strings.Split(smtpServer, ":")[0])
	if err := client.Auth(auth); err != nil {
		log.Println("Error authenticating:", err)
		http.Error(w, "Error sending email, error authenticating", http.StatusInternalServerError)
		return
	}

	if err := client.Mail(fromEmail); err != nil {
		log.Println("Error setting sender:", err)
		http.Error(w, "Error sending email, error setting sender", http.StatusInternalServerError)
		return
	}

	if err := client.Rcpt(toEmail); err != nil {
		log.Println("Error setting recipient:", err)
		http.Error(w, "Error sending email, error setting recipient", http.StatusInternalServerError)
		return
	}

	data, err := client.Data()
	if err != nil {
		log.Println("Error sending email body:", err)
		http.Error(w, "Error sending email, no body?", http.StatusInternalServerError)
		return
	}
	defer data.Close()

	subject := "New Account Request!"
	body := fmt.Sprintf("Username: %s\nEmail: %s", requestData.Username, requestData.Email)
	msg := fmt.Sprintf("Subject: %s\n%s\n", subject, body)

	_, err = data.Write([]byte(msg))
	if err != nil {
		log.Println("Error writing email data:", err)
		http.Error(w, "Error sending email, error writing data", http.StatusInternalServerError)
		return
	}

	if err := client.Quit(); err != nil {
		log.Println("Error sending email:", err)
		http.Error(w, "Error sending email, could not send?", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Email sent successfully")
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/request", sendEmailHandler)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Origin", "Authorization", "Content-Type"},
	})

	handler := c.Handler(mux)

	log.Fatal(http.ListenAndServe(":8080", handler))
}

func dialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	return &connWithTimeout{Conn: conn, timeout: timeout}, nil
}

type connWithTimeout struct {
	net.Conn
	timeout time.Duration
}

func (c *connWithTimeout) Read(b []byte) (int, error) {
	c.SetReadDeadline(time.Now().Add(c.timeout))
	return c.Conn.Read(b)
}

func (c *connWithTimeout) Write(b []byte) (int, error) {
	c.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.Conn.Write(b)
}

func sendEmailWithTimeout(addr string, a smtp.Auth, from string, to []string, msg []byte, timeout time.Duration) error {
	conn, err := dialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, addr)
	if err != nil {
		return err
	}
	defer c.Quit()

	if a != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(a); err != nil {
				return err
			}
		}
	}

	if err = c.Mail(from); err != nil {
		return err
	}

	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}

	return c.Quit()
}

