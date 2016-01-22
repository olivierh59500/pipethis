package main

import (
	"bufio"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/ellotheth/pipethis/lookup"
)

type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			os.Exit(1)
		}
	}()

	var (
		target   = flag.String("target", os.Getenv("SHELL"), "Executable to run the script")
		inspect  = flag.Bool("inspect", false, "Open an editor to inspect the file before running it")
		editor   = flag.String("editor", os.Getenv("EDITOR"), "Editor to inspect the script")
		noVerify = flag.Bool("no-verify", false, "Don't verify the author or signature")
	)
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		log.Panic("No script specified")
	}

	if _, err := os.Stat(*target); os.IsNotExist(err) {
		log.Panic("Script executable does not exist")
	}
	log.Println("Using script executable", *target)

	// download the script, store it someplace temporary
	script, err := NewScript(flag.Arg(0))
	if err != nil {
		log.Panic(err)
	}
	defer os.Remove(script.Name())
	log.Println("Script saved to", script.Name())

	if cont := script.Inspect(*inspect, *editor); !cont {
		log.Panic("Exiting without running", script.Name())
	}

	if !*noVerify {
		author, err := script.Author()
		if err != nil {
			log.Panic(err)
		}

		key, err := lookup.Key(lookup.KeybaseService{}, author)
		if err != nil {
			log.Panic(err)
		}

		signature := NewSignature(key, script)
		defer os.Remove(signature.Name())

		if err := signature.Verify(); err != nil {
			log.Panic(err)
		}

		log.Println("Signature", signature.Source(), "verified!")
	}

	// run the script
	log.Println("Running", script.Name(), "with", *target)
	script.Run(*target, flag.Args()...)
}

func parseToken(pattern string, reader io.Reader) string {
	re := regexp.MustCompile(pattern)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		matches := re.FindStringSubmatch(scanner.Text())
		if matches == nil {
			continue
		}

		return matches[1]
	}

	return ""
}

func getFile(location string) (io.ReadCloser, error) {
	body, err := getLocal(location)
	if err == nil {
		return body, nil
	}

	return getRemote(location)
}

func getRemote(location string) (io.ReadCloser, error) {
	parsed, err := url.Parse(location)
	if err != nil || parsed.Scheme == "" {
		return nil, errors.New("Invalid URL")
	}

	resp, err := http.Get(location)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func getLocal(location string) (io.ReadCloser, error) {
	if _, err := os.Stat(location); os.IsNotExist(err) {
		return nil, err
	}

	return os.Open(location)
}
