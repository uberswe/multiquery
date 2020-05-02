package cmd

import (
	"bytes"
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/go-sql-driver/mysql"
	"github.com/kevinburke/ssh_config"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"path"
	"strings"
	"syscall"
)

var rootCmd = &cobra.Command{
	Use:   "mq",
	Short: "mq or MultiQuery is a cli tool to perform mysql queries on multiple databases",
	Long:  "mq or MultiQuery is a cli tool to perform mysql queries on multiple databases\n\nCreated by Markus Tenghamn (m@rkus.io)",
	Run:   run,
}

type ViaSSHDialer struct {
	client *ssh.Client
	_      *context.Context
}

func (d *ViaSSHDialer) Dial(ctx context.Context, addr string) (net.Conn, error) {
	return d.client.Dial("tcp", addr)
}

func run(cmd *cobra.Command, args []string) {
	if sshHost != "" {
		runOverSSH(runMysql, "mysql+tcp")
	} else {
		cfg, err := ini.Load(dbConf)
		if err == nil {
			loadMyCnf(cfg)
		}
		runMysql("tcp")
	}
	log.Println("done")
}

func runMysql(dbNet string) {
	var databasesToQuery []string
	if db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@%s(%s:%s)/%s?parseTime=true", dbUser, dbPass, dbNet, dbHost, dbPort, dbName)); err == nil {
		defer db.Close()
		if rows, err := db.Query("SHOW DATABASES"); err == nil {
			for rows.Next() {
				var databaseName string
				_ = rows.Scan(&databaseName)
				if dbPrefix != "" {
					if strings.HasPrefix(databaseName, dbPrefix) && !strings.Contains(databaseName, dbIgnore) {
						databasesToQuery = append(databasesToQuery, databaseName)
					}
				} else {
					databasesToQuery = append(databasesToQuery, databaseName)
				}

			}
		} else {
			log.Fatal(err)
		}
	}

	// Run the query
	if dbQuery != "" {
		queries := make(chan string)
		var executedDbs []string
		for _, databaseName := range databasesToQuery {
			if threaded {
				go executeThreadedQuery(dbNet, databaseName, queries)
			} else {
				executeQuery(dbNet, databaseName)
			}
		}
		for {
			msg := <-queries
			executedDbs = append(executedDbs, msg)
			if len(executedDbs) == len(databasesToQuery) {
				break
			}
		}
	}
}

func executeThreadedQuery(dbNet string, databaseName string, queries chan string) {
	executeQuery(dbNet, databaseName)
	queries <- databaseName
}

func executeQuery(dbNet string, databaseName string) {
	if db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@%s(%s:%s)/%s?parseTime=true", dbUser, dbPass, dbNet, dbHost, dbPort, databaseName)); err == nil {
		defer db.Close()
		if rows, err := db.Query(dbQuery); err == nil {
			cols, err := rows.Columns()
			if err != nil {
				fmt.Println("Failed to get columns", err)
				return
			}

			// Result is your slice string.
			rawResult := make([][]byte, len(cols))
			result := make([]string, len(cols))

			dest := make([]interface{}, len(cols)) // A temporary interface{} slice
			for i, _ := range rawResult {
				dest[i] = &rawResult[i] // Put pointers to each string in the interface slice
			}

			for rows.Next() {
				err = rows.Scan(dest...)
				if err != nil {
					fmt.Println("Failed to scan row", err)
					return
				}

				for i, raw := range rawResult {
					if raw == nil {
						result[i] = "\\N"
					} else {
						result[i] = string(raw)
					}
				}

				fmt.Printf("%s: %#v\n", databaseName, result)
			}
		} else {
			log.Fatal(err)
		}
	}

}

func runOverSSH(mysqlFunc func(dbNet string), dbNet string) {

	// Attempt to read sshEnabled config file
	sshHostName := ssh_config.Get(sshHost, "HostName")
	sshPort := ssh_config.Get(sshHost, "Port")
	sshIdentityFile := ssh_config.Get(sshHost, "IdentityFile")
	sshUser := ssh_config.Get(sshHost, "User")

	if sshHostName != "" {
		sshHost = sshHostName
	}
	if sshPort == "" {
		sshPort = sshPort
	}
	if sshIdentityFile != "~/.sshEnabled/identity" && privkeyPath == defaultKeyPath {
		privkeyPath = sshIdentityFile
	}
	if sshUser != "root" && sshUser != "" {
		sshUser = sshUser
	}

	if strings.Contains(privkeyPath, "~/") {
		usr, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}

		privkeyPath = path.Join(usr.HomeDir, strings.Replace(privkeyPath, "~", "", 1))
	}

	sshHost = fmt.Sprintf("%s:%s", sshHost, sshPort)

	key, err := ioutil.ReadFile(privkeyPath)
	if err != nil {
		log.Fatalf("Unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		CheckPassword()
		der := decrypt(key, password)
		key, err := x509.ParsePKCS1PrivateKey(der)
		if err != nil {
			log.Fatalf("Unable to parse private key: %v", err)
		}
		signer, err = ssh.NewSignerFromKey(key)
		if err != nil {
			log.Fatalf("Unable to get signer from private key: %v", err)
		}

	}

	var agentClient agent.Agent
	// Establish a connection to the local ssh-agent
	if conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		defer conn.Close()

		// Create a new instance of the ssh agent
		agentClient = agent.NewClient(conn)
	}

	// The client configuration with configuration option to use the ssh-agent
	sshConfig := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// When the agentClient connection succeeded, add them as AuthMethod
	if agentClient != nil {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(agentClient.Signers))
	}
	// When there's a non empty password add the password AuthMethod
	if sshPass != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PasswordCallback(func() (string, error) {
			return sshPass, nil
		}))
	}

	// Connect to the SSH Server
	if sshcon, err := ssh.Dial("tcp", sshHost, sshConfig); err == nil {
		defer sshcon.Close()

		session, _ := sshcon.NewSession()
		defer session.Close()

		var stdoutBuf bytes.Buffer
		session.Stdout = &stdoutBuf
		session.Run(fmt.Sprintf("cat %s", dbConf))
		mycnfBytes := stdoutBuf.Bytes()

		if len(mycnfBytes) > 0 {
			cfg, err := ini.Load(mycnfBytes)
			if err != nil {
				panic(err)
			}
			loadMyCnf(cfg)
		}

		dialer := ViaSSHDialer{client: sshcon}
		// Now we register the ViaSSHDialer with the ssh connection as a parameter
		mysql.RegisterDialContext("mysql+tcp", dialer.Dial)
		// once we are connected to mysql over ssh we can run the mysql stuff as we normally would
		mysqlFunc(dbNet)
	} else {
		log.Fatal(err)
	}

}

func loadMyCnf(cfg *ini.File) {
	for _, s := range cfg.Sections() {
		if s.Key("host").String() != "" && s.Key("host").String() != dbHost {
			dbHost = s.Key("host").String()
		}
		if s.Key("port").String() != "" && s.Key("port").String() != dbPort {
			dbPort = s.Key("port").String()
		}
		if s.Key("dbname").String() != "" && s.Key("dbname").String() != dbName {
			dbName = s.Key("dbname").String()
		}
		if s.Key("user").String() != "" && s.Key("user").String() != dbUser {
			dbUser = s.Key("user").String()
		}
		if s.Key("password").String() != "" && s.Key("password").String() != dbPass {
			dbPass = s.Key("password").String()
		}
	}
}

func CheckPassword() {
	if password == nil {
		var err error
		fmt.Print("Enter ssh key Password: ")
		password, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Fatalf("Invalid password: %v", err)
		}
	}
}

func decrypt(key []byte, password []byte) []byte {
	block, rest := pem.Decode(key)
	if len(rest) > 0 {
		log.Fatalf("Extra data included in key")
	}
	der, err := x509.DecryptPEMBlock(block, password)
	if err != nil {
		log.Fatalf("Decrypt failed: %v", err)
	}
	return der
}
