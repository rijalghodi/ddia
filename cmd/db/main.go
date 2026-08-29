package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	appendonly "github.com/rijalghodi/ddia/03-storage-retrieval/appendonlylog"
	segmented "github.com/rijalghodi/ddia/03-storage-retrieval/segmentedlog"
)

type Database interface {
	Put(key string, value string) error
	Get(key string) (string, bool, error)
	Delete(key string) error
	Close() error
}

func main() {
	enginePtr := flag.String("engine", "", "Database engine to use: 'appendonly' or 'segmented'")
	flag.Parse()

	engineName := *enginePtr
	if engineName == "" {
		fmt.Print("Choose database engine [segmented, appendonly] (default: segmented): ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input == "" {
				engineName = "segmented"
			} else {
				engineName = input
			}
		} else {
			engineName = "segmented"
		}
	}

	var db Database
	var err error

	// 1. Initialize the correct engine based on the flag
	switch engineName {
	case "appendonly":
		db, err = appendonly.Open("./data/appendonly.log")
	case "segmented":
		db, err = segmented.Open("./data/segmented", 1024*1024, 3) // 1MB size, 3 segments
	default:
		fmt.Printf("Error: Unknown engine '%s'.\n", engineName)
		fmt.Println("Valid engines are 'appendonly' or 'segmented'.")
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Printf("Started DDIA Database using engine: '%s'\n", engineName)
	printAvailableCommand()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Split by spaces, max 3 parts so the value can contain spaces
		parts := strings.SplitN(line, " ", 3)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "put":
			if len(parts) < 3 {
				fmt.Println("Usage: put <key> <value>")
				continue
			}
			err := db.Put(parts[1], parts[2])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("OK")
			}
		case "get":
			if len(parts) < 2 {
				fmt.Println("Usage: get <key>")
				continue
			}
			val, ok, err := db.Get(parts[1])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else if !ok {
				fmt.Println("Not found")
			} else {
				fmt.Println(val)
			}
		case "delete":
			if len(parts) < 2 {
				fmt.Println("Usage: delete <key>")
				continue
			}
			err := db.Delete(parts[1])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("OK")
			}
		case "help":
			printAvailableCommand()
		case "quit", "exit":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Printf("Unknown command: %s (type 'help' for available commands)\n", cmd)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading standard input: %v\n", err)
	}
}

func printAvailableCommand() {
	fmt.Println("Available commands:")
	fmt.Println("  put <key> <value>")
	fmt.Println("  get <key>")
	fmt.Println("  delete <key>")
	fmt.Println("  help")
	fmt.Println("  quit")
}
