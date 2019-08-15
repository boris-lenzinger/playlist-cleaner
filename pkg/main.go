package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/eiannone/keyboard"
)

var mplayer *exec.Cmd
var stdinMplayer io.WriteCloser
var currentReadVideo string
var toDelete map[string]string

func playVideo(pathToVideo string, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)
	mplayer = exec.Command("/usr/bin/mplayer", pathToVideo)
	mplayer.Start()

	waitGroup.Done()
}

func stopPlayVideo() {
	mplayer.Process.Kill()
}

func getPrev(list []string, currentIndex int) string {
	if currentIndex > 0 && len(list) > currentIndex {
		return list[currentIndex-1]
	}
	return ""
}

func getNext(list []string, currentIndex int) string {
	if currentIndex < len(list)-2 {
		return list[currentIndex+1]
	}
	return ""
}

func showDetailsOfDeletion(playlist []string, toDelete map[string]string) {
	totalSize := int64(0)

	var f *os.File
	var fi os.FileInfo
	for _, path := range playlist {
		if _, ok := toDelete[path]; ok {
			f, _ = os.Open(path)
			fi, _ = f.Stat()
			s := fi.Size()
			fmt.Printf(" * %s %s \n", sizeToMegaOrGiga(s), path)
			totalSize += s
		}
	}
	if totalSize > 0 {
		fmt.Printf("                 Total Size: %s \n", sizeToMegaOrGiga(totalSize))
	}
}

func sizeToMegaOrGiga(size int64) string {
	sizeInMega := float64(size) / 1024 / 1024
	sizeInGiga := float64(sizeInMega) / 1024

	if sizeInGiga < 1 {
		return fmt.Sprintf("%d Mb", int64(sizeInMega))
	}
	return fmt.Sprintf("%.3f Gb", sizeInGiga)
}

func main() {
	toDelete := make(map[string]string)
	playlist := os.Args[1]
	f, err := os.Open(playlist)
	if err != nil {
		log.Printf("Error: failed to open playlist %q due to %s", playlist, err)
		os.Exit(1)
	}
	videosListAsBytes, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("Error: failed to read playlist %q due to %s", playlist, err)
		os.Exit(1)
	}

	err = keyboard.Open()
	if err != nil {
		panic(err)
	}
	defer keyboard.Close()

	videosListAsString := string(videosListAsBytes)
	videosList := strings.Split(videosListAsString, "\n")
	indexInPlaylist := 0
	var currentVideo, nextVideo, prevVideo string

	for {
		log.Println(" ================================================== ")
		log.Printf("         Video %d/%d\n", indexInPlaylist+1, len(videosList))
		log.Println(" ================================================== ")
		currentVideo = videosList[indexInPlaylist]
		prevVideo = getPrev(videosList, indexInPlaylist)
		nextVideo = getNext(videosList, indexInPlaylist)
		var waitgroup sync.WaitGroup
		log.Printf("Reading video %q\n", currentVideo)
		if _, ok := toDelete[currentVideo]; ok {
			fmt.Println()
			log.Println("       [WARNING] This video has been marked to be deleted")
			fmt.Println()
		}
		go playVideo(currentVideo, &waitgroup)

		log.Println("Available commands: ")
		log.Println("  * > : switch to next video")
		log.Println("  * < : switch to previous video")
		log.Println("  * d : mark video as to be deleted")
		log.Println("  * u : removes video from list to be deleted")
		log.Println("  * q : quit process. Videos marked as deleted will be proposed for deletion")
		log.Println()
		log.Printf("Number of videos marked as to be deleted: %q\n", len(toDelete))

		var s string
		for {
			char, key, err := keyboard.GetKey()
			if err != nil {
				panic(err)
			} else if key == keyboard.KeyEsc {
				break
			}
			s = string(char)
			switch s {
			case ">":
				if nextVideo == "" {
					log.Printf("Already at the end of the playlist. Exiting")
					stopPlayVideo()
					goto endReading
				} else {
					log.Printf("Switching to next file %s\n", nextVideo)
					stopPlayVideo()
					indexInPlaylist++
					goto playNextVideo
				}
			case "<":
				if indexInPlaylist > 0 {
					log.Printf("Switching to previous file %s\n", prevVideo)
					indexInPlaylist--
					stopPlayVideo()
					goto playNextVideo
				} else {
					log.Println("Already at the beginning of the playlist. There is no previous file.")
				}
			case "d":
				log.Printf("Requesting to delete %s\n", currentVideo)
				toDelete[currentVideo] = currentVideo
			case "u":
				log.Printf("Requesting to undelete %s\n", currentVideo)
				delete(toDelete, currentVideo)
			case "q":
				log.Printf("Quit reading. Stopped at video %d\n", indexInPlaylist+1)
				stopPlayVideo()
				goto endReading
			default:
				log.Printf("Command %q is unsupported. Please use <,>,d,u,q instead.\n", s)
			}
			log.Printf("[logger] You pressed: %q\r\n", char)
		}
	playNextVideo:

		waitgroup.Wait()
	}
endReading:

	showDetailsOfDeletion(videosList, toDelete)
}
