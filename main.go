// Package main is the main starting point of the program. It parses
// the command line arguments and initializes the mysql connection pool.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/MeesCode/mmjs/audioplayer"
	"github.com/MeesCode/mmjs/database"
	"github.com/MeesCode/mmjs/globals"
	"github.com/MeesCode/mmjs/plugins"
	"github.com/MeesCode/mmjs/tui"
)

var (
	modes      = []string{"filesystem", "database", "index"}
	help       bool
	configFile string
)

func init() {
	var (
		defaultMode             = "filesystem"
		defaultHelp             = false
		defaultQuiet            = false
		defaultWebserver        = false
		defaultLogging          = false
		defaultWebserverPort    = 8080
		defaultDatabasePort     = 3306
		defaultDatabaseHost     = "localhost"
		defaultDatabaseUser     = ""
		defaultDatabasePassword = ""
		defaultDatabase         = ""
		defaultConfig           = ""
		defaultDisableSound     = false
		defaultHighlight        = "cb2821"

		modeUsage             = "specifies what mode to run. [" + strings.Join(modes, ", ") + "]"
		webserverUsage        = "a boolean to specify whether to run the webserver. (only in database mode)"
		quietUsage            = "quiet mode disables the text user interface"
		loggingUsage          = "enable error logging in stdout, will interfere with tui"
		helpUsage             = "print this help message"
		webserverPortUsage    = "set the port to be used by the webserver plugin"
		databasePortUsage     = "set the port to be used by the database"
		databaseHostUsage     = "set the host for the database connection"
		databaseUserUsage     = "set the database user"
		databasePasswordUsage = "set the database password"
		databaseUsage         = "The database to use"
		disableSoundUsage     = "disables initialization of the sound card (for server use)"
		configUsage           = "specify a config file to use (overrides command line arguments)"
		highlightUsage        = "hex code (ffffff) indicating the highlight color of the text user interface"
	)

	flag.BoolVar(&help, "help", defaultHelp, helpUsage)
	flag.StringVar(&configFile, "c", defaultConfig, configUsage)
	flag.StringVar(&globals.Config.Mode, "m", defaultMode, modeUsage)
	flag.StringVar(&globals.Config.Highlight, "hl", defaultHighlight, highlightUsage)
	flag.BoolVar(&globals.Config.Quiet, "q", defaultQuiet, quietUsage)
	flag.BoolVar(&globals.Config.Logging, "x", defaultLogging, loggingUsage)
	flag.BoolVar(&globals.Config.Webserver.Enable, "w", defaultWebserver, webserverUsage)
	flag.IntVar(&globals.Config.Webserver.Port, "wp", defaultWebserverPort, webserverPortUsage)
	flag.IntVar(&globals.Config.Database.Port, "dp", defaultDatabasePort, databasePortUsage)
	flag.StringVar(&globals.Config.Database.Host, "h", defaultDatabaseHost, databaseHostUsage)
	flag.StringVar(&globals.Config.Database.User, "u", defaultDatabaseUser, databaseUserUsage)
	flag.StringVar(&globals.Config.Database.Password, "p", defaultDatabasePassword, databasePasswordUsage)
	flag.StringVar(&globals.Config.Database.Database, "d", defaultDatabase, databaseUsage)
	flag.BoolVar(&globals.Config.DisableSound, "ds", defaultDisableSound, disableSoundUsage)
}

// load the configuration from a json file
func loadConfiguration(file string) globals.ConfigFile {
	var config globals.ConfigFile
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		log.Fatalln("could not open configuration file (config.json)", err)
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		log.Fatalln("error decoding configuration file", err)
	}
	return config
}

func main() {

	// disable logs by default
	log.SetOutput(ioutil.Discard)

	// parse command line arguments
	flag.Parse()

	// check for help flag
	if help {
		flag.PrintDefaults()
		return
	}

	// load configuration file if one is specified
	if configFile != "" {
		globals.Config = loadConfiguration(configFile)
	}

	base, err := os.Getwd()

	if err != nil {
		log.Fatalln("could not get working directory", err)
	}

	arg := flag.Arg(0)

	// check that a path has been given
	if arg == "" {
		fmt.Println("please specify a path")
		return
	}

	// make absolute if not already
	if path.IsAbs(arg) {
		globals.Root = path.Clean(arg)
	} else {
		globals.Root = path.Clean(path.Join(base, arg))
	}

	// check if mode is correct
	if !globals.Contains(modes, globals.Config.Mode) {
		fmt.Println("please use one of the available modes")
		flag.PrintDefaults()
		return
	}

	// check if path exists
	if _, err := os.Stat(globals.Root); os.IsNotExist(err) {
		fmt.Println("chosen path: " + globals.Root)
		fmt.Println("specified path does not exist")
		flag.PrintDefaults()
		return
	}

	// disabe logging output. discard instead
	if globals.Config.Logging {
		log.SetOutput(os.Stdout)
	}

	// index filesystem at specified path
	if globals.Config.Mode == "index" {
		db := database.Warmup()
		defer db.Close()
		database.Index()
		return
	}

	// start the databse connection pool
	if globals.Config.Mode != "filesystem" {
		db := database.Warmup()
		defer db.Close()
	}

	// initialize audio player
	if !globals.Config.DisableSound {
		go audioplayer.Initialize()
	}

	////////////////////////////////
	//     Start plugins here     //
	////////////////////////////////

	// the webserver relies heavily on the search function which, while
	// functional is increadibly slow outside of database mode
	if globals.Config.Webserver.Enable && globals.Config.Mode == "database" {
		go plugins.Webserver()
	}

	///////////////////////////////
	//  Begin main program loop  //
	///////////////////////////////

	if !globals.Config.Quiet {
		// start user interface
		// on current thread as to not immediately exit
		tui.Start()
	} else {
		// idle until sigterm is caught
		fmt.Println("started in quiet mode")
		sigs := make(chan os.Signal, 1)

		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs

		audioplayer.Stop()
	}

}
