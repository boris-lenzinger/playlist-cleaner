package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/eiannone/keyboard"
)

func playVideo(pathToVideo string, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)
	output, err := exec.Command("/usr/bin/mplayer",  pathToVideo).Output()
	if err != nil {
		fmt.Printf("Erreur lors de la lecture du fichier: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", string(output))

	waitGroup.Done()
}

func main() {
	videoFile := os.Args[1]
	var waitgroup sync.WaitGroup
	go playVideo(videoFile, &waitgroup)

	err := keyboard.Open()
	if err != nil {
		panic(err)
	}
	defer keyboard.Close()

	fmt.Println("Press ESC to quit")
	for {
		char, key, err := keyboard.GetKey()
		if (err != nil) {
			panic(err)
		} else if (key == keyboard.KeyEsc) {
			break
		}
		fmt.Printf("You pressed: %q\r\n", char)
	}

	waitgroup.Wait()
}