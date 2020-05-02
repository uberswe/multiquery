package cmd

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path"
)

var (
	threaded       bool
	sshHost        string
	sshPort        string
	sshUser        string
	sshPass        string
	privkeyPath    string
	defaultKeyPath string
	password       []byte
	dbUser         string
	dbPass         string
	dbHost         string
	dbPort         string
	dbName         string
	dbPrefix       string
	dbIgnore       string
	dbConf         string
	dbQuery        string
)

func Execute() {

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	defaultKeyPath = path.Join(usr.HomeDir, ".ssh/id_rsa")

	rootCmd.PersistentFlags().StringVarP(&dbQuery, "query", "q", "", "Mysql query you would like to run")
	rootCmd.PersistentFlags().StringVarP(&dbUser, "user", "u", "", "Mysql user")
	rootCmd.PersistentFlags().StringVarP(&dbPass, "password", "p", "", "Mysql password")
	rootCmd.PersistentFlags().StringVar(&dbHost, "host", "", "Mysql host")
	rootCmd.PersistentFlags().StringVar(&dbPort, "port", "3306", "Mysql port")
	rootCmd.PersistentFlags().StringVarP(&dbName, "database", "d", "", "Mysql database")
	rootCmd.PersistentFlags().StringVar(&dbPrefix, "dbprefix", "", "Mysql prefix for matching multiple database names")
	rootCmd.PersistentFlags().StringVar(&dbIgnore, "dbignore", "", "Ignores any database containing this string")
	rootCmd.PersistentFlags().StringVarP(&dbConf, "conf", "c", "~/.my.cnf", "Mysql config file location")
	rootCmd.PersistentFlags().StringVar(&sshHost, "sshhost", "", "SSH host")
	rootCmd.PersistentFlags().StringVar(&sshPort, "sshport", "", "SSH port to connect to (default is 22)")
	rootCmd.PersistentFlags().StringVar(&sshUser, "sshuser", "root", "SSH user")
	rootCmd.PersistentFlags().StringVar(&sshPass, "sshpass", "", "SSH password")
	rootCmd.PersistentFlags().BoolVar(&threaded, "threaded", false, "Use threading to run queries in parallel")
	rootCmd.PersistentFlags().StringVar(&privkeyPath, "sshkey", defaultKeyPath, "Path to your SSH private key")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
